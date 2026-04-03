package cmd

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/parser"
	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index a codebase and build its dependency graph",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runIndex,
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

	scipFiles, err := collectSCIPIndexes(absPath, outDir, files)
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

func collectSCIPIndexes(root, outDir string, files map[parser.Language][]string) ([]string, error) {
	scipFile := filepath.Join(root, "index.scip")
	if _, err := os.Stat(scipFile); err == nil {
		return []string{scipFile}, nil
	}
	return generateSCIPIndexes(root, filepath.Join(outDir, "scip"), files, scipFile)
}

func generateSCIPIndexes(root, scipOutDir string, files map[parser.Language][]string, scipFile string) ([]string, error) {
	candidates := []struct {
		name    string
		output  string
		args    []string
		markers []string
		enabled bool
	}{
		{
			name:    "scip-go",
			output:  "scip-go.scip",
			markers: []string{"go.mod"},
			enabled: len(files[parser.LangGo]) > 0,
		},
		{
			name:    "scip-typescript",
			output:  "scip-typescript.scip",
			args:    []string{"index"},
			markers: []string{"package.json"},
			enabled: len(files[parser.LangTypeScript]) > 0 || len(files[parser.LangJavaScript]) > 0,
		},
		{
			name:    "scip-python",
			output:  "scip-python.scip",
			args:    []string{"index", ".", "--project-name", filepath.Base(root)},
			markers: []string{"pyproject.toml", "requirements.txt"},
			enabled: len(files[parser.LangPython]) > 0,
		},
	}

	var generated []string
	for _, candidate := range candidates {
		if !candidate.enabled || !hasAnyMarker(root, candidate.markers) {
			continue
		}
		if candidate.name == "scip-python" && runtime.GOOS == "windows" {
			log.Warn().
				Str("indexer", candidate.name).
				Msg("skipping scip-python on Windows because the published package fails on native Windows path separators; use WSL/Linux CI for Python SCIP indexing")
			continue
		}
		if _, err := scipLookPath(candidate.name); err != nil {
			continue
		}
		if err := os.MkdirAll(scipOutDir, 0755); err != nil {
			return generated, fmt.Errorf("creating SCIP output dir: %w", err)
		}
		if err := os.Remove(scipFile); err != nil && !os.IsNotExist(err) {
			return generated, fmt.Errorf("removing stale root SCIP index: %w", err)
		}
		if err := runSCIPCommand(root, candidate.name, candidate.args...); err != nil {
			log.Warn().Err(err).Str("indexer", candidate.name).Msg("SCIP indexer failed")
			continue
		}
		if _, err := os.Stat(scipFile); err != nil {
			log.Warn().Str("indexer", candidate.name).Str("path", scipFile).Msg("SCIP indexer completed without producing index.scip")
			continue
		}
		targetPath := filepath.Join(scipOutDir, candidate.output)
		if err := os.Rename(scipFile, targetPath); err != nil {
			return generated, fmt.Errorf("saving %s output: %w", candidate.name, err)
		}
		generated = append(generated, targetPath)
	}

	return generated, nil
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
