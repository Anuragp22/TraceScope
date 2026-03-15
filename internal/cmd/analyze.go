package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/anurag/tracescope/internal/analyzer"
	diffpkg "github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/output"
	"github.com/spf13/cobra"
)

var diffFile string

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze blast radius of changes from a diff",
	Long:  "Accepts a unified diff via --diff flag or stdin (e.g., git diff | tracescope analyze)",
	RunE:  runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringVar(&diffFile, "diff", "", "path to a unified diff file")
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	// Read diff from file or stdin
	var diffData []byte
	var err error

	if diffFile != "" {
		diffData, err = os.ReadFile(diffFile)
		if err != nil {
			return fmt.Errorf("reading diff file: %w", err)
		}
	} else {
		// Check if stdin has data
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return fmt.Errorf("no diff provided — use --diff <file> or pipe via stdin (e.g., git diff | tracescope analyze)")
		}
		diffData, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
	}

	if len(diffData) == 0 {
		return fmt.Errorf("empty diff")
	}

	// Parse diff
	changedFiles, err := diffpkg.ParseUnifiedDiff(diffData)
	if err != nil {
		return fmt.Errorf("parsing diff: %w", err)
	}

	// Load graph
	cwd, _ := os.Getwd()
	graphFile := filepath.Join(cwd, ".tracescope", "graph.json")
	store := graph.NewStore()
	graphData, err := store.Load(graphFile)
	if err != nil {
		return fmt.Errorf("loading graph (run 'tracescope index' first): %w", err)
	}

	// Run blast radius analysis
	ba := analyzer.NewBlastRadiusAnalyzer(graphData)
	result := ba.Analyze(changedFiles)

	// Output
	output.PrintAnalysis(result)

	return nil
}
