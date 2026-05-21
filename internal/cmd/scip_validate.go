package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/parser"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var scipValidateCmd = &cobra.Command{
	Use:           "validate-scip [path]",
	Short:         "Compare SCIP graph output against parser fallback graph output",
	Args:          cobra.MaximumNArgs(1),
	RunE:          runSCIPValidate,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	rootCmd.AddCommand(scipValidateCmd)
}

func runSCIPValidate(cmd *cobra.Command, args []string) error {
	targetPath := "."
	if len(args) > 0 {
		targetPath = args[0]
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	cyan := color.New(color.FgCyan, color.Bold)
	cyan.Fprintf(os.Stderr, "TraceScope")
	fmt.Fprintf(os.Stderr, " - validating SCIP graph quality for %s\n\n", absPath)

	files, err := parser.WalkDirectory(absPath)
	if err != nil {
		return fmt.Errorf("walking directory: %w", err)
	}

	scipFiles, _, err := collectSCIPIndexes(absPath, filepath.Join(absPath, ".tracescope"), files)
	if err != nil {
		return fmt.Errorf("collecting SCIP indexes: %w", err)
	}
	if len(scipFiles) == 0 {
		return fmt.Errorf("no SCIP indexes available for %s", absPath)
	}

	scipGraph, err := graph.BuildFromSCIPFiles(scipFiles)
	if err != nil {
		return fmt.Errorf("building SCIP graph: %w", err)
	}

	registry := parser.NewRegistry()
	results, parseErrs := registry.ParseFiles(files)
	for _, parseErr := range parseErrs {
		fmt.Fprintf(os.Stderr, "  parser warning: %v\n", parseErr)
	}
	parserGraph := graph.NewBuilder().Build(results)

	comparison := graph.CompareGraphShape(parserGraph, scipGraph, absPath, 10)
	fmt.Fprintf(os.Stderr, "  Nodes: parser=%d scip=%d shared=%d missing=%d extra=%d\n",
		comparison.BaseNodeCount,
		comparison.CandidateNodeCount,
		comparison.SharedNodeCount,
		comparison.MissingNodeCount,
		comparison.ExtraNodeCount,
	)
	fmt.Fprintf(os.Stderr, "  Edges: parser=%d scip=%d shared=%d missing=%d extra=%d\n",
		comparison.BaseEdgeCount,
		comparison.CandidateEdgeCount,
		comparison.SharedEdgeCount,
		comparison.MissingEdgeCount,
		comparison.ExtraEdgeCount,
	)

	if len(comparison.MissingNodes) > 0 {
		fmt.Fprintln(os.Stderr, "\n  Missing parser nodes in SCIP:")
		for _, signature := range comparison.MissingNodes {
			fmt.Fprintf(os.Stderr, "    - %s\n", signature)
		}
	}
	if len(comparison.MissingEdges) > 0 {
		fmt.Fprintln(os.Stderr, "\n  Missing parser edges in SCIP:")
		for _, signature := range comparison.MissingEdges {
			fmt.Fprintf(os.Stderr, "    - %s\n", signature)
		}
	}
	if len(comparison.ExtraEdges) > 0 {
		fmt.Fprintln(os.Stderr, "\n  Extra SCIP edges:")
		for _, signature := range comparison.ExtraEdges {
			fmt.Fprintf(os.Stderr, "    - %s\n", signature)
		}
	}

	return nil
}
