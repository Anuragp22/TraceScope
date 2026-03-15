package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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
	bold := color.New(color.Bold)
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

	// Step 2: Parse files
	log.Debug().Msg("parsing files")
	registry := parser.NewRegistry()
	results, errs := registry.ParseFiles(files)
	if len(errs) > 0 {
		for _, e := range errs {
			log.Warn().Err(e).Msg("parse error")
		}
	}
	fmt.Fprintf(os.Stderr, "  Parsed %d files (%d errors)\n", len(results), len(errs))

	// Step 3: Build graph
	log.Debug().Msg("building graph")
	builder := graph.NewBuilder()
	graphData := builder.Build(results)

	nodeCount := len(graphData.Nodes)
	edgeCount := len(graphData.Edges)
	fmt.Fprintf(os.Stderr, "  Built graph: %d nodes, %d edges\n", nodeCount, edgeCount)

	// Step 4: Save graph
	store := graph.NewStore()
	outDir := filepath.Join(absPath, ".tracescope")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	outFile := filepath.Join(outDir, "graph.json")
	if err := store.Save(graphData, outFile); err != nil {
		return fmt.Errorf("saving graph: %w", err)
	}

	elapsed := time.Since(start)

	fmt.Fprintln(os.Stderr)
	bold.Fprintf(os.Stderr, "  Stats:\n")

	// Count by node type
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

	fmt.Fprintf(os.Stderr, "\n  Saved to %s\n", outFile)
	fmt.Fprintf(os.Stderr, "  Done in %s\n", elapsed.Round(time.Millisecond))

	return nil
}
