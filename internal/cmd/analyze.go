package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/anurag/tracescope/internal/analyzer"
	diffpkg "github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/output"
	"github.com/spf13/cobra"
)

var (
	diffFile      string
	analysisDepth int
	outputFormat  string
	topN          int
	ignoreGlobs   string
	githubComment bool
)

var analyzeCmd = &cobra.Command{
	Use:           "analyze",
	Short:         "Analyze blast radius of changes from a diff",
	Long:          "Accepts a unified diff via --diff flag or stdin (e.g., git diff | tracescope analyze)",
	RunE:          runAnalyze,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	analyzeCmd.Flags().StringVar(&diffFile, "diff", "", "path to a unified diff file")
	analyzeCmd.Flags().IntVar(&analysisDepth, "depth", 5, "maximum blast radius depth")
	analyzeCmd.Flags().StringVar(&outputFormat, "format", "terminal", "output format: terminal or json")
	analyzeCmd.Flags().IntVar(&topN, "top", 0, "show only top N affected functions (0 = all)")
	analyzeCmd.Flags().StringVar(&ignoreGlobs, "ignore", "", "comma-separated glob patterns to exclude files (e.g., vendor/**,dist/**)")
	analyzeCmd.Flags().BoolVar(&githubComment, "github-comment", false, "post blast radius as a GitHub PR comment (requires gh CLI)")
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
		stat, err := os.Stdin.Stat()
		if err != nil {
			return fmt.Errorf("no diff provided — use --diff <file> or pipe via stdin (e.g., git diff | tracescope analyze)")
		}
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

	// Apply ignore patterns
	if ignoreGlobs != "" {
		changedFiles = filterIgnoredFiles(changedFiles, ignoreGlobs)
		if len(changedFiles) == 0 {
			return fmt.Errorf("all changed files were excluded by --ignore patterns")
		}
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
	ba := analyzer.NewBlastRadiusAnalyzer(graphData, analysisDepth)
	result := ba.Analyze(changedFiles)

	// Apply --top N pagination
	if topN > 0 && topN < len(result.AffectedFunctions) {
		result.TotalAffected = len(result.AffectedFunctions)
		result.AffectedFunctions = result.AffectedFunctions[:topN]
		result.TopN = topN
	}

	// Output
	switch outputFormat {
	case "json":
		if err := output.PrintJSON(result); err != nil {
			return fmt.Errorf("writing JSON: %w", err)
		}
	default:
		output.PrintAnalysis(result)
	}

	// Post GitHub comment if requested
	if githubComment {
		if err := output.PostGitHubComment(result); err != nil {
			return fmt.Errorf("GitHub comment: %w", err)
		}
		fmt.Fprintln(os.Stderr, "  Posted blast radius comment to PR.")
	}

	// Exit with risk-based code
	if code := analyzer.HighestRiskExitCode(result); code > 0 {
		return &analyzer.RiskExitError{Code: code}
	}

	return nil
}

// filterIgnoredFiles removes files matching any of the comma-separated glob patterns.
func filterIgnoredFiles(files []diffpkg.ChangedFile, patterns string) []diffpkg.ChangedFile {
	globs := strings.Split(patterns, ",")
	for i := range globs {
		globs[i] = strings.TrimSpace(globs[i])
	}

	var result []diffpkg.ChangedFile
	for _, f := range files {
		if matchesAnyGlob(f.Path, globs) {
			continue
		}
		result = append(result, f)
	}
	return result
}

// matchesAnyGlob checks if a path matches any of the glob patterns.
// Supports ** as a prefix match (e.g., "vendor/**" matches "vendor/foo/bar.go").
func matchesAnyGlob(path string, globs []string) bool {
	normalized := filepath.ToSlash(path)
	for _, g := range globs {
		if g == "" {
			continue
		}
		// Handle ** patterns as prefix match
		if strings.HasSuffix(g, "/**") {
			prefix := strings.TrimSuffix(g, "/**")
			if strings.HasPrefix(normalized, prefix+"/") || normalized == prefix {
				return true
			}
			continue
		}
		// Standard glob match on the filename
		if matched, _ := filepath.Match(g, filepath.Base(normalized)); matched {
			return true
		}
		// Also try matching against the full path
		if matched, _ := filepath.Match(g, normalized); matched {
			return true
		}
	}
	return false
}
