package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/anurag/tracescope/internal/analyzer"
	diffpkg "github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/output"
	"github.com/anurag/tracescope/internal/ownership"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	diffFile      string
	analysisDepth int
	outputFormat  string
	topN          int
	ignoreGlobs   string
	githubComment bool
	showOwners    bool
	allowStale    bool
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
	analyzeCmd.Flags().BoolVar(&showOwners, "owners", false, "show code owners and last authors for affected code")
	analyzeCmd.Flags().BoolVar(&allowStale, "allow-stale", false, "analyze even when the graph was indexed at a different commit than HEAD")
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	// Merge config defaults with CLI flags (CLI takes precedence)
	if !cmd.Flags().Changed("depth") && cfg.MaxDepth > 0 {
		analysisDepth = cfg.MaxDepth
	}
	if !cmd.Flags().Changed("format") && cfg.Format != "" {
		outputFormat = cfg.Format
	}
	if !cmd.Flags().Changed("top") && cfg.TopN > 0 {
		topN = cfg.TopN
	}
	// Merge ignore patterns: config + CLI (union)
	if len(cfg.Ignore) > 0 {
		configIgnore := strings.Join(cfg.Ignore, ",")
		if ignoreGlobs != "" {
			ignoreGlobs = ignoreGlobs + "," + configIgnore
		} else {
			ignoreGlobs = configIgnore
		}
	}

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
	graphData, err := loadGraph()
	if err != nil {
		return fmt.Errorf("loading graph (run 'tracescope index' first): %w", err)
	}

	// A graph built at another revision has drifted line numbers, so the diff
	// maps onto whatever now occupies those lines. The result is not degraded,
	// it is wrong — and wrong in a way that looks exactly like a correct answer.
	//
	// This used to be a warning on stderr, which in CI scrolls past unread. It
	// now stops the run: exit code 3, the code that already means "TraceScope
	// could not do its job", as distinct from 1 and 2 which mean "your change is
	// risky". --allow-stale is there for the case where you know the drift does
	// not matter and want an answer anyway.
	if graphData.Commit != "" {
		if head := gitHead("."); head != "" && head != graphData.Commit {
			if !allowStale {
				return fmt.Errorf(
					"graph was indexed at %s but HEAD is %s — line numbers have drifted, "+
						"so the diff would map onto the wrong functions.\n"+
						"       Re-run 'tracescope index' to refresh it, or pass --allow-stale to analyze anyway",
					shortSHA(graphData.Commit), shortSHA(head))
			}
			log.Warn().
				Str("graph_commit", shortSHA(graphData.Commit)).
				Str("head", shortSHA(head)).
				Msg("analysing against a stale graph because --allow-stale was passed; line mapping may be wrong")
		}
	}

	// Run blast radius analysis
	scorer := &analyzer.RiskScorer{
		HighCallers:         cfg.Risk.HighCallers,
		HighExportedCallers: cfg.Risk.HighExportedCallers,
		MediumCallers:       cfg.Risk.MediumCallers,
	}
	ba := analyzer.NewBlastRadiusAnalyzer(graphData, analysisDepth, scorer)
	result := ba.Analyze(changedFiles)

	// Ownership lookup
	if showOwners {
		repoRoot, err := findRepoRoot()
		if err != nil {
			log.Warn().Err(err).Msg("could not find git repo root, skipping ownership")
		} else {
			ownerInfo, err := ownership.ResolveOwnership(repoRoot, result.AffectedFunctions, result.ChangedFiles)
			if err != nil {
				log.Warn().Err(err).Msg("ownership lookup failed")
			} else {
				result.Ownership = ownerInfo
				for i := range result.AffectedFunctions {
					af := &result.AffectedFunctions[i]
					if af.Node != nil {
						if info, ok := ownerInfo.FileAuthors[af.Node.FilePath]; ok {
							af.LastAuthor = info.LastAuthor
							af.LastEmail = info.LastEmail
							af.LastModified = info.LastModified.Format(time.RFC3339)
						}
					}
				}
			}
		}
	}

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

	// Post GitHub comment if requested. A failure here must NOT mask the risk
	// signal: the exit code is the CI contract, the comment is a side effect. So
	// degrade gracefully (a transient gh/API hiccup shouldn't read as "the tool
	// failed" and block a merge) and let the risk-based exit code below stand.
	if githubComment {
		if err := output.PostGitHubComment(result); err != nil {
			log.Warn().Err(err).Msg("failed to post GitHub comment; continuing")
		} else {
			fmt.Fprintln(os.Stderr, "  Posted blast radius comment to PR.")
		}
	}

	// Exit with risk-based code
	if code := analyzer.HighestRiskExitCode(result); code > 0 {
		return &analyzer.RiskExitError{Code: code}
	}

	return nil
}
