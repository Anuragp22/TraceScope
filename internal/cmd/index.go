package cmd

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/parser"
	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	scip "github.com/scip-code/scip/bindings/go/scip"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var indexCmd = &cobra.Command{
	Use:           "index [path]",
	Short:         "Index a codebase and build its dependency graph",
	Args:          cobra.MaximumNArgs(1),
	RunE:          runIndex,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	rootCmd.AddCommand(indexCmd)
}

func runIndex(cmd *cobra.Command, args []string) error {
	targetPath := "."
	if len(args) > 0 {
		targetPath = args[0]
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	start := time.Now()
	cyan := color.New(color.FgCyan, color.Bold)

	cyan.Fprintf(os.Stderr, "TraceScope")
	fmt.Fprintf(os.Stderr, " — indexing %s\n\n", absPath)

	// Step 1: Walk and discover files
	log.Debug().Str("path", absPath).Msg("walking directory")
	files, err := parser.WalkDirectory(absPath)
	if err != nil {
		return fmt.Errorf("walking directory: %w", err)
	}

	totalFiles := 0
	for lang, langFiles := range files {
		count := len(langFiles)
		totalFiles += count
		log.Debug().Str("language", string(lang)).Int("files", count).Msg("discovered files")
	}
	fmt.Fprintf(os.Stderr, "  Found %d files across %d languages\n", totalFiles, len(files))

	// Step 2: Check for incremental indexing
	outDir := filepath.Join(absPath, ".tracescope")
	graphFile := filepath.Join(outDir, "graph.json")
	cacheFile := filepath.Join(outDir, "parse_cache.json")

	scipFiles, indexerStatuses, err := collectSCIPIndexes(absPath, outDir, files)
	if err != nil {
		log.Warn().Err(err).Msg("SCIP generation failed, falling back to parser")
	}
	if len(scipFiles) > 0 {
		for _, scipFile := range scipFiles {
			fmt.Fprintf(os.Stderr, "  Using SCIP index: %s\n", scipFile)
		}

		graphData, err := graph.BuildFromSCIPFiles(scipFiles)
		if err != nil {
			return fmt.Errorf("loading SCIP index: %w", err)
		}
		graphData.RootPath = absPath
		graphData.IndexerStatuses = indexerStatuses

		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
		if err := storeGraph(graphFile, graphData); err != nil {
			return err
		}

		printIndexStats(graphData, 0, 0, time.Since(start), graphFile)
		return nil
	}

	store := graph.NewStore()
	existingGraph, _ := store.Load(graphFile)
	cache, _ := parser.LoadCache(cacheFile)

	// Determine which files need re-parsing
	var filesToParse map[parser.Language][]string
	var cachedResults []*parser.FileResult
	reParsed := 0
	cached := 0

	if existingGraph != nil && existingGraph.FileMetadata != nil && len(cache.Results) > 0 {
		// Incremental mode
		filesToParse = make(map[parser.Language][]string)
		for lang, langFiles := range files {
			for _, f := range langFiles {
				meta, hasMeta := existingGraph.FileMetadata[f]
				cachedResult, hasCached := cache.Results[f]

				if hasMeta && hasCached {
					// Check hash
					source, err := os.ReadFile(f)
					if err != nil {
						filesToParse[lang] = append(filesToParse[lang], f)
						reParsed++
						continue
					}
					h := sha256.Sum256(source)
					hash := fmt.Sprintf("%x", h)
					if hash == meta.Hash {
						cachedResults = append(cachedResults, cachedResult)
						cached++
						continue
					}
				}
				filesToParse[lang] = append(filesToParse[lang], f)
				reParsed++
			}
		}
	} else {
		// Full index mode
		filesToParse = files
		reParsed = totalFiles
	}

	// Step 3: Parse files (only changed ones)
	log.Debug().Msg("parsing files")
	registry := parser.NewRegistry()
	freshResults, errs := registry.ParseFiles(filesToParse)
	if len(errs) > 0 {
		for _, e := range errs {
			log.Warn().Err(e).Msg("parse error")
		}
	}

	// Merge fresh + cached results
	allResults := append(cachedResults, freshResults...)

	if cached > 0 {
		fmt.Fprintf(os.Stderr, "  Parsed %d files (%d cached, %d errors)\n", len(freshResults), cached, len(errs))
	} else {
		fmt.Fprintf(os.Stderr, "  Parsed %d files (%d errors)\n", len(allResults), len(errs))
	}

	// Step 4: Build graph from ALL results
	log.Debug().Msg("building graph")
	builder := graph.NewBuilder()
	graphData := builder.Build(allResults)
	graphData.RootPath = absPath
	graphData.IndexSource = "parser"
	graphData.IndexerStatuses = indexerStatuses

	// Step 5: Update parse cache
	newCache := parser.NewParseCache()
	for _, r := range allResults {
		newCache.Results[r.FilePath] = r
	}
	// Remove deleted files from cache
	currentFiles := make(map[string]bool)
	for _, langFiles := range files {
		for _, f := range langFiles {
			currentFiles[f] = true
		}
	}
	for path := range newCache.Results {
		if !currentFiles[path] {
			delete(newCache.Results, path)
		}
	}

	// Step 6: Save graph + cache
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := storeGraph(graphFile, graphData); err != nil {
		return err
	}
	if err := parser.SaveCache(newCache, cacheFile); err != nil {
		log.Warn().Err(err).Msg("failed to save parse cache")
	}

	printIndexStats(graphData, reParsed, cached, time.Since(start), graphFile)

	return nil
}

var scipLookPath = exec.LookPath

var runSCIPCommand = func(dir, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	return command.Run()
}

type scipIndexerCandidate struct {
	name       string
	workDir    string
	output     string
	pathPrefix string
	args       []string
	markers    []string
	sourceList []string
	enabled    bool
}

func collectSCIPIndexes(root, outDir string, files map[parser.Language][]string) ([]string, []graph.IndexerStatus, error) {
	scipFile := filepath.Join(root, "index.scip")
	if _, err := os.Stat(scipFile); err == nil {
		return []string{scipFile}, []graph.IndexerStatus{{
			Name:   "index.scip",
			State:  "used_existing",
			Output: scipFile,
		}}, nil
	}
	return generateSCIPIndexes(root, filepath.Join(outDir, "scip"), files, scipFile)
}

func generateSCIPIndexes(root, scipOutDir string, files map[parser.Language][]string, scipFile string) ([]string, []graph.IndexerStatus, error) {
	candidates := buildSCIPIndexerCandidates(root, files)

	var generated []string
	statuses := make([]graph.IndexerStatus, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.enabled {
			statuses = append(statuses, graph.IndexerStatus{
				Name:   candidate.name,
				State:  "skipped",
				Reason: "no matching source files",
			})
			continue
		}
		if !hasAnyMarker(candidate.workDir, candidate.markers) {
			statuses = append(statuses, graph.IndexerStatus{
				Name:   candidate.name,
				State:  "skipped",
				Reason: fmt.Sprintf("missing project marker (%v)", candidate.markers),
			})
			continue
		}
		targetPath := filepath.Join(scipOutDir, candidate.output)
		if scipIndexCacheFresh(targetPath, candidate.workDir, candidate.sourceList, candidate.markers) {
			if err := rewriteSCIPDocumentPaths(targetPath, candidate.pathPrefix); err != nil {
				return generated, statuses, fmt.Errorf("rewriting cached %s document paths: %w", candidate.name, err)
			}
			generated = append(generated, targetPath)
			statuses = append(statuses, graph.IndexerStatus{
				Name:   candidate.name,
				State:  "cached",
				Output: targetPath,
			})
			continue
		}
		if candidate.name == "scip-python" && runtime.GOOS == "windows" {
			log.Warn().
				Str("indexer", candidate.name).
				Msg("skipping scip-python on Windows because the published package fails on native Windows path separators; use WSL/Linux CI for Python SCIP indexing")
			statuses = append(statuses, graph.IndexerStatus{
				Name:   candidate.name,
				State:  "skipped",
				Reason: "scip-python is disabled on native Windows; use WSL/Linux CI",
			})
			continue
		}
		if _, err := scipLookPath(scipCommandName(candidate.name)); err != nil {
			statuses = append(statuses, graph.IndexerStatus{
				Name:   candidate.name,
				State:  "missing_binary",
				Reason: err.Error(),
			})
			continue
		}
		if err := os.MkdirAll(scipOutDir, 0755); err != nil {
			return generated, statuses, fmt.Errorf("creating SCIP output dir: %w", err)
		}
		candidateSCIPFile := filepath.Join(candidate.workDir, "index.scip")
		if err := os.Remove(candidateSCIPFile); err != nil && !os.IsNotExist(err) {
			return generated, statuses, fmt.Errorf("removing stale root SCIP index: %w", err)
		}
		if err := runSCIPCommand(candidate.workDir, scipCommandName(candidate.name), candidate.args...); err != nil {
			log.Warn().Err(err).Str("indexer", candidate.name).Msg("SCIP indexer failed")
			statuses = append(statuses, graph.IndexerStatus{
				Name:   candidate.name,
				State:  "failed",
				Reason: err.Error(),
			})
			continue
		}
		if _, err := os.Stat(candidateSCIPFile); err != nil {
			log.Warn().Str("indexer", candidate.name).Str("path", candidateSCIPFile).Msg("SCIP indexer completed without producing index.scip")
			statuses = append(statuses, graph.IndexerStatus{
				Name:   candidate.name,
				State:  "failed",
				Reason: "indexer completed without producing index.scip",
			})
			continue
		}
		if err := rewriteSCIPDocumentPaths(candidateSCIPFile, candidate.pathPrefix); err != nil {
			return generated, statuses, fmt.Errorf("rewriting %s document paths: %w", candidate.name, err)
		}
		if err := os.Rename(candidateSCIPFile, targetPath); err != nil {
			return generated, statuses, fmt.Errorf("saving %s output: %w", candidate.name, err)
		}
		generated = append(generated, targetPath)
		statuses = append(statuses, graph.IndexerStatus{
			Name:      candidate.name,
			State:     "generated",
			Output:    targetPath,
			Generated: true,
		})
	}

	return generated, statuses, nil
}

func buildSCIPIndexerCandidates(root string, files map[parser.Language][]string) []scipIndexerCandidate {
	candidates := []scipIndexerCandidate{{
		name:       "scip-go",
		workDir:    root,
		output:     "scip-go.scip",
		markers:    []string{"go.mod"},
		sourceList: files[parser.LangGo],
		enabled:    len(files[parser.LangGo]) > 0,
	}}

	jsTsSources := append(append([]string{}, files[parser.LangTypeScript]...), files[parser.LangJavaScript]...)
	tsProjectRoots := groupSourcesByNearestMarkerRoot(root, jsTsSources, "package.json")
	if len(tsProjectRoots) == 0 {
		candidates = append(candidates, scipIndexerCandidate{
			name:       "scip-typescript",
			workDir:    root,
			output:     "scip-typescript.scip",
			args:       []string{"index"},
			markers:    []string{"package.json"},
			sourceList: jsTsSources,
			enabled:    len(jsTsSources) > 0,
		})
	} else {
		roots := sortedMapKeys(tsProjectRoots)
		for _, projectRoot := range roots {
			candidateName := "scip-typescript"
			outputName := "scip-typescript.scip"
			pathPrefix := ""
			if rel, err := filepath.Rel(root, projectRoot); err == nil && rel != "." {
				pathPrefix = canonicalRelativePath(rel)
				slug := strings.ReplaceAll(pathPrefix, "/", "-")
				candidateName += "@" + pathPrefix
				outputName = "scip-typescript-" + slug + ".scip"
			}
			candidates = append(candidates, scipIndexerCandidate{
				name:       candidateName,
				workDir:    projectRoot,
				output:     outputName,
				pathPrefix: pathPrefix,
				args:       []string{"index"},
				markers:    []string{"package.json"},
				sourceList: tsProjectRoots[projectRoot],
				enabled:    len(tsProjectRoots[projectRoot]) > 0,
			})
		}
	}

	candidates = append(candidates, scipIndexerCandidate{
		name:       "scip-python",
		workDir:    root,
		output:     "scip-python.scip",
		args:       []string{"index", ".", "--project-name", filepath.Base(root)},
		markers:    []string{"pyproject.toml", "requirements.txt"},
		sourceList: files[parser.LangPython],
		enabled:    len(files[parser.LangPython]) > 0,
	})
	return candidates
}

func groupSourcesByNearestMarkerRoot(root string, sourceFiles []string, marker string) map[string][]string {
	grouped := map[string][]string{}
	for _, sourceFile := range sourceFiles {
		projectRoot := nearestMarkerRoot(root, sourceFile, marker)
		if projectRoot == "" {
			continue
		}
		grouped[projectRoot] = append(grouped[projectRoot], sourceFile)
	}
	return grouped
}

func nearestMarkerRoot(root, sourceFile, marker string) string {
	root = filepath.Clean(root)
	dir := filepath.Dir(filepath.Clean(sourceFile))
	for {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return dir
		}
		if dir == root {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func sortedMapKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func scipCommandName(candidateName string) string {
	if name, _, ok := strings.Cut(candidateName, "@"); ok {
		return name
	}
	return candidateName
}

func canonicalRelativePath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func rewriteSCIPDocumentPaths(indexPath, prefix string) error {
	if prefix == "" || prefix == "." {
		return nil
	}

	raw, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("reading SCIP index: %w", err)
	}

	var index scip.Index
	if err := proto.Unmarshal(raw, &index); err != nil {
		return fmt.Errorf("decoding SCIP index: %w", err)
	}

	for _, doc := range index.GetDocuments() {
		relPath := canonicalRelativePath(doc.GetRelativePath())
		if relPath == "" || relPath == "." || strings.HasPrefix(relPath, prefix+"/") {
			continue
		}
		doc.RelativePath = canonicalRelativePath(filepath.Join(prefix, relPath))
	}

	updated, err := proto.Marshal(&index)
	if err != nil {
		return fmt.Errorf("encoding SCIP index: %w", err)
	}
	if err := os.WriteFile(indexPath, updated, 0o600); err != nil {
		return fmt.Errorf("writing SCIP index: %w", err)
	}
	return nil
}

func scipIndexCacheFresh(indexPath, root string, sourceFiles, markers []string) bool {
	indexInfo, err := os.Stat(indexPath)
	if err != nil {
		return false
	}
	indexTime := indexInfo.ModTime()

	for _, sourceFile := range sourceFiles {
		if sourceInfo, err := os.Stat(sourceFile); err != nil || sourceInfo.ModTime().After(indexTime) {
			return false
		}
	}
	for _, marker := range markers {
		markerInfo, err := os.Stat(filepath.Join(root, marker))
		if err == nil && markerInfo.ModTime().After(indexTime) {
			return false
		}
	}
	return true
}

func hasAnyMarker(root string, markers []string) bool {
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(root, marker)); err == nil {
			return true
		}
	}
	return false
}

func storeGraph(graphFile string, graphData *graph.GraphData) error {
	store := graph.NewStore()
	if err := store.Save(graphData, graphFile); err != nil {
		return fmt.Errorf("saving graph: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  Built graph: %d nodes, %d edges\n", len(graphData.Nodes), len(graphData.Edges))
	return nil
}

func printIndexStats(graphData *graph.GraphData, reParsed, cached int, elapsed time.Duration, graphFile string) {
	bold := color.New(color.Bold)
	dim := color.New(color.Faint)

	fmt.Fprintln(os.Stderr)
	bold.Fprintf(os.Stderr, "  Stats:\n")

	if graphData.IndexSource != "" {
		fmt.Fprintf(os.Stderr, "    %-12s %s\n", "source:", graphData.IndexSource)
	}
	if len(graphData.IndexerStatuses) > 0 {
		fmt.Fprintln(os.Stderr, "    indexers:")
		for _, status := range graphData.IndexerStatuses {
			switch {
			case status.Output != "":
				fmt.Fprintf(os.Stderr, "      - %s: %s (%s)\n", status.Name, status.State, status.Output)
			case status.Reason != "":
				fmt.Fprintf(os.Stderr, "      - %s: %s (%s)\n", status.Name, status.State, status.Reason)
			default:
				fmt.Fprintf(os.Stderr, "      - %s: %s\n", status.Name, status.State)
			}
		}
	}

	typeCounts := map[graph.NodeType]int{}
	for _, n := range graphData.Nodes {
		typeCounts[n.Type]++
	}
	for t, c := range typeCounts {
		fmt.Fprintf(os.Stderr, "    %-12s %d\n", t+":", c)
	}

	edgeTypeCounts := map[graph.EdgeType]int{}
	for _, e := range graphData.Edges {
		edgeTypeCounts[e.Type]++
	}
	for t, c := range edgeTypeCounts {
		fmt.Fprintf(os.Stderr, "    %-12s %d\n", string(t)+":", c)
	}

	if cached > 0 {
		fmt.Fprintln(os.Stderr)
		dim.Fprintf(os.Stderr, "  Incremental: %d files re-parsed, %d cached\n", reParsed, cached)
	}

	fmt.Fprintf(os.Stderr, "\n  Saved to %s\n", graphFile)
	fmt.Fprintf(os.Stderr, "  Done in %s\n", elapsed.Round(time.Millisecond))
}
