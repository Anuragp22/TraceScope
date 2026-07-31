package output

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/ownership"
	"github.com/fatih/color"
)

// reportOut is where a human-readable report goes. It is stdout, not stderr:
// the report is this tool's output, so `tracescope analyze > review.txt` has to
// capture it. Progress lines, warnings and diagnostics about the run itself
// still go to stderr, which is what keeps a piped report clean.
var reportOut io.Writer = os.Stdout

var (
	bold   = color.New(color.Bold)
	cyan   = color.New(color.FgCyan, color.Bold)
	red    = color.New(color.FgRed, color.Bold)
	yellow = color.New(color.FgYellow, color.Bold)
	green  = color.New(color.FgGreen, color.Bold)
	dim    = color.New(color.Faint)
)

// PrintAnalysis outputs the blast radius analysis to the terminal.
func PrintAnalysis(result *analyzer.AnalysisResult) {
	cwd, _ := os.Getwd()

	fmt.Fprintln(reportOut)
	cyan.Fprintln(reportOut, "TraceScope - Blast Radius Analysis")
	fmt.Fprintln(reportOut)

	bold.Fprintf(reportOut, "  Changed Files (%d):\n", len(result.ChangedFiles))
	for _, cf := range result.ChangedFiles {
		label := ""
		if cf.IsNew {
			label = " [NEW]"
		} else if cf.IsDeleted {
			label = " [DELETED]"
		}
		fmt.Fprintf(reportOut, "    %s%s\n", cf.Path, label)
	}
	fmt.Fprintln(reportOut)

	bold.Fprintf(reportOut, "  Changed Functions (%d):\n", len(result.ChangedFunctions))
	for _, cf := range result.ChangedFunctions {
		relPath := shortPathCwd(cf.FilePath, cwd)
		fmt.Fprintf(reportOut, "    %s ", cf.Node.Name)
		dim.Fprintf(reportOut, "(%s:%d)\n", relPath, cf.Node.StartLine)
	}
	fmt.Fprintln(reportOut)

	high, medium, low := groupByRisk(result.AffectedFunctions)

	bold.Fprintf(reportOut, "  Blast Radius (%d affected):\n", len(result.AffectedFunctions))
	fmt.Fprintln(reportOut)

	if len(high) > 0 {
		red.Fprintf(reportOut, "    HIGH RISK (%d):\n", len(high))
		printAffected(high, cwd)
		fmt.Fprintln(reportOut)
	}

	if len(medium) > 0 {
		yellow.Fprintf(reportOut, "    MEDIUM RISK (%d):\n", len(medium))
		printAffected(medium, cwd)
		fmt.Fprintln(reportOut)
	}

	if len(low) > 0 {
		green.Fprintf(reportOut, "    LOW RISK (%d):\n", len(low))
		printAffected(low, cwd)
		fmt.Fprintln(reportOut)
	}

	if len(result.AffectedFunctions) == 0 {
		dim.Fprintln(reportOut, "    No affected functions found in blast radius.")
		fmt.Fprintln(reportOut)
	}

	if result.TopN > 0 && result.TotalAffected > result.TopN {
		dim.Fprintf(reportOut, "    ... showing top %d of %d affected functions\n\n", result.TopN, result.TotalAffected)
	}

	bold.Fprintln(reportOut, "  Summary:")
	fmt.Fprintf(reportOut, "    Graph: %d nodes, %d edges\n", result.TotalNodes, result.TotalEdges)
	fmt.Fprintf(reportOut, "    Changed: %d files, %d functions\n", len(result.ChangedFiles), len(result.ChangedFunctions))
	fmt.Fprintf(reportOut, "    Blast radius: %d affected functions (depth %d)\n", len(result.AffectedFunctions), result.MaxDepth)
	fmt.Fprintf(reportOut, "    Confidence: %d exact, %d heuristic, %d ambiguous skipped, %d unresolved\n",
		result.ResolutionStats.ExactCallEdges,
		result.ResolutionStats.HeuristicCallEdges,
		result.ResolutionStats.AmbiguousCalls,
		result.ResolutionStats.UnresolvedCalls,
	)
	fmt.Fprintf(reportOut, "    Risk: ")
	red.Fprintf(reportOut, "%d high", len(high))
	fmt.Fprintf(reportOut, ", ")
	yellow.Fprintf(reportOut, "%d medium", len(medium))
	fmt.Fprintf(reportOut, ", ")
	green.Fprintf(reportOut, "%d low", len(low))
	fmt.Fprintln(reportOut)

	if result.Ownership != nil {
		if oi, ok := result.Ownership.(*ownership.OwnershipInfo); ok && len(oi.SuggestedReviewers) > 0 {
			fmt.Fprintln(reportOut)
			bold.Fprintln(reportOut, "  Suggested Reviewers:")
			for _, r := range oi.SuggestedReviewers {
				fmt.Fprintf(reportOut, "    %s ", r.Owner)
				dim.Fprintf(reportOut, "(%d file%s)\n", r.FileCount, pluralS(r.FileCount))
			}
		}
	}

	if len(result.ResolutionIssues) > 0 {
		fmt.Fprintln(reportOut)
		bold.Fprintln(reportOut, "  Resolution Diagnostics:")
		printResolutionIssues(result.ResolutionIssues, cwd)
	}

	fmt.Fprintln(reportOut)
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func groupByRisk(funcs []analyzer.AffectedFunction) (high, medium, low []analyzer.AffectedFunction) {
	for _, f := range funcs {
		switch f.Risk {
		case analyzer.RiskHigh:
			high = append(high, f)
		case analyzer.RiskMedium:
			medium = append(medium, f)
		case analyzer.RiskLow:
			low = append(low, f)
		}
	}

	sortGroup := func(s []analyzer.AffectedFunction) {
		sort.Slice(s, func(i, j int) bool {
			if s[i].ReviewScore != s[j].ReviewScore {
				return s[i].ReviewScore > s[j].ReviewScore
			}
			if s[i].CallerCount != s[j].CallerCount {
				return s[i].CallerCount > s[j].CallerCount
			}
			if s[i].Node == nil || s[j].Node == nil {
				return s[i].Node != nil
			}
			return s[i].Node.Name < s[j].Node.Name
		})
	}
	sortGroup(high)
	sortGroup(medium)
	sortGroup(low)

	return
}

func printAffected(funcs []analyzer.AffectedFunction, cwd string) {
	for _, f := range funcs {
		if f.Node == nil {
			continue
		}
		relPath := shortPathCwd(f.Node.FilePath, cwd)
		fmt.Fprintf(reportOut, "      %s ", f.Node.Name)
		dim.Fprintf(reportOut, "(%s:%d) ", relPath, f.Node.StartLine)
		// CallerCount is every caller including tests; the reason string quotes
		// production callers only. Labelling both "callers" put two different
		// numbers under the same word on one line.
		dim.Fprintf(reportOut, "[score %d, %d total callers, depth %d, %s, %s]", f.ReviewScore, f.CallerCount, f.Depth, formatConfidence(f.Confidence), f.Reason)
		if f.LastAuthor != "" {
			dim.Fprintf(reportOut, " by %s", f.LastAuthor)
		}
		fmt.Fprintln(reportOut)
		if path := formatImpactPathText(f); path != "" {
			dim.Fprintf(reportOut, "        path: %s\n", path)
		}
	}
}

func printResolutionIssues(issues []graph.ResolutionIssue, cwd string) {
	limit := len(issues)
	if limit > 10 {
		limit = 10
	}
	for i := 0; i < limit; i++ {
		issue := issues[i]
		path := shortPathCwd(issue.FilePath, cwd)
		if issue.Line > 0 {
			path = fmt.Sprintf("%s:%d", path, issue.Line)
		}
		symbol := issue.Symbol
		if issue.Receiver != "" {
			symbol = issue.Receiver + "." + issue.Symbol
		}
		if symbol == "" {
			symbol = "-"
		}
		dim.Fprintf(reportOut, "    %s %s %s (%s)\n", issue.Status, issue.Kind, symbol, path)
		if issue.Detail != "" {
			dim.Fprintf(reportOut, "      %s\n", issue.Detail)
		}
	}
	if len(issues) > limit {
		dim.Fprintf(reportOut, "    ... showing first %d of %d diagnostics\n", limit, len(issues))
	}
}

func formatImpactPathText(f analyzer.AffectedFunction) string {
	if len(f.ImpactPath) == 0 {
		return ""
	}
	parts := make([]string, 0, len(f.ImpactPath))
	for _, step := range f.ImpactPath {
		if step.Node == nil {
			continue
		}
		parts = append(parts, step.Node.Name)
	}
	return strings.Join(parts, " -> ")
}

func shortPathCwd(p string, cwd string) string {
	if cwd == "" {
		return p
	}
	rel, err := filepath.Rel(cwd, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}
