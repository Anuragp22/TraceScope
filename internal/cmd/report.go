package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/anurag/tracescope/internal/analyzer"
	diffpkg "github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/output"
	"github.com/spf13/cobra"
)

var (
	reportDiffFile string
	reportDepth    int
	reportOutput   string
	reportOpen     bool
	reportIgnore   string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate an interactive HTML dependency graph report",
	Long: `Creates a self-contained HTML file with an interactive D3.js force-directed
dependency graph. Optionally overlay blast radius analysis from a diff.

Examples:
  tracescope report                          # graph-only report
  tracescope report --open                   # generate and open in browser
  tracescope report --diff changes.patch     # with blast radius overlay
  git diff | tracescope report --open        # pipe diff and open`,
	RunE:          runReport,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	reportCmd.Flags().StringVar(&reportDiffFile, "diff", "", "path to unified diff file (enables blast radius overlay)")
	reportCmd.Flags().IntVar(&reportDepth, "depth", 5, "blast radius depth (used with --diff)")
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "tracescope-report.html", "output HTML file path")
	reportCmd.Flags().BoolVar(&reportOpen, "open", false, "open report in default browser after generation")
	reportCmd.Flags().StringVar(&reportIgnore, "ignore", "", "comma-separated glob patterns to exclude files")
	rootCmd.AddCommand(reportCmd)
}

func runReport(cmd *cobra.Command, args []string) error {
	// Merge config
	if !cmd.Flags().Changed("depth") && cfg.MaxDepth > 0 {
		reportDepth = cfg.MaxDepth
	}

	// Load graph
	graphData, err := loadGraph()
	if err != nil {
		return fmt.Errorf("loading graph (run 'tracescope index' first): %w", err)
	}

	// Optionally run blast radius analysis
	var analysis *analyzer.AnalysisResult
	diffData, hasDiff := readDiffInput(reportDiffFile)
	if hasDiff && len(diffData) > 0 {
		changedFiles, err := diffpkg.ParseUnifiedDiff(diffData)
		if err != nil {
			return fmt.Errorf("parsing diff: %w", err)
		}

		// Apply ignore patterns
		allIgnore := reportIgnore
		if len(cfg.Ignore) > 0 {
			if allIgnore != "" {
				allIgnore += "," + strings.Join(cfg.Ignore, ",")
			} else {
				allIgnore = strings.Join(cfg.Ignore, ",")
			}
		}
		if allIgnore != "" {
			changedFiles = filterIgnoredFiles(changedFiles, allIgnore)
		}

		if len(changedFiles) > 0 {
			scorer := &analyzer.RiskScorer{
				HighCallers:         cfg.Risk.HighCallers,
				HighExportedCallers: cfg.Risk.HighExportedCallers,
				MediumCallers:       cfg.Risk.MediumCallers,
			}
			ba := analyzer.NewBlastRadiusAnalyzer(graphData, reportDepth, scorer)
			analysis = ba.Analyze(changedFiles)
		}
	}

	// Generate report
	f, err := os.Create(reportOutput)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	if err := output.GenerateReport(f, graphData, analysis); err != nil {
		return fmt.Errorf("generating report: %w", err)
	}

	abs, _ := os.Getwd()
	fmt.Fprintf(os.Stderr, "  Report saved to %s/%s\n", abs, reportOutput)

	if analysis != nil {
		fmt.Fprintf(os.Stderr, "  Includes blast radius overlay (%d affected functions)\n", len(analysis.AffectedFunctions))
	}

	// Open in browser
	if reportOpen {
		openBrowser(reportOutput)
	}

	return nil
}

// readDiffInput tries to read diff from file or stdin.
// Returns the data and whether a diff source was available.
func readDiffInput(diffFile string) ([]byte, bool) {
	if diffFile != "" {
		data, err := os.ReadFile(diffFile)
		if err != nil {
			return nil, false
		}
		return data, true
	}

	// Check stdin
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, false
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil, false
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

func openBrowser(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}
