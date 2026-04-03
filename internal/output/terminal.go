package output

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/ownership"
	"github.com/fatih/color"
)

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

	fmt.Fprintln(os.Stderr)
	cyan.Fprintln(os.Stderr, "TraceScope - Blast Radius Analysis")
	fmt.Fprintln(os.Stderr)

	bold.Fprintf(os.Stderr, "  Changed Files (%d):\n", len(result.ChangedFiles))
	for _, cf := range result.ChangedFiles {
		label := ""
		if cf.IsNew {
			label = " [NEW]"
		} else if cf.IsDeleted {
			label = " [DELETED]"
		}
		fmt.Fprintf(os.Stderr, "    %s%s\n", cf.Path, label)
	}
	fmt.Fprintln(os.Stderr)

	bold.Fprintf(os.Stderr, "  Changed Functions (%d):\n", len(result.ChangedFunctions))
	for _, cf := range result.ChangedFunctions {
		relPath := shortPathCwd(cf.FilePath, cwd)
		fmt.Fprintf(os.Stderr, "    %s ", cf.Node.Name)
		dim.Fprintf(os.Stderr, "(%s:%d)\n", relPath, cf.Node.StartLine)
	}
	fmt.Fprintln(os.Stderr)

	high, medium, low := groupByRisk(result.AffectedFunctions)

	bold.Fprintf(os.Stderr, "  Blast Radius (%d affected):\n", len(result.AffectedFunctions))
	fmt.Fprintln(os.Stderr)

	if len(high) > 0 {
		red.Fprintf(os.Stderr, "    HIGH RISK (%d):\n", len(high))
		printAffected(high, cwd)
		fmt.Fprintln(os.Stderr)
	}

	if len(medium) > 0 {
		yellow.Fprintf(os.Stderr, "    MEDIUM RISK (%d):\n", len(medium))
		printAffected(medium, cwd)
		fmt.Fprintln(os.Stderr)
	}

	if len(low) > 0 {
		green.Fprintf(os.Stderr, "    LOW RISK (%d):\n", len(low))
		printAffected(low, cwd)
		fmt.Fprintln(os.Stderr)
	}

	if len(result.AffectedFunctions) == 0 {
		dim.Fprintln(os.Stderr, "    No affected functions found in blast radius.")
		fmt.Fprintln(os.Stderr)
	}

	if result.TopN > 0 && result.TotalAffected > result.TopN {
		dim.Fprintf(os.Stderr, "    ... showing top %d of %d affected functions\n\n", result.TopN, result.TotalAffected)
	}

	bold.Fprintln(os.Stderr, "  Summary:")
	fmt.Fprintf(os.Stderr, "    Graph: %d nodes, %d edges\n", result.TotalNodes, result.TotalEdges)
	fmt.Fprintf(os.Stderr, "    Changed: %d files, %d functions\n", len(result.ChangedFiles), len(result.ChangedFunctions))
	fmt.Fprintf(os.Stderr, "    Blast radius: %d affected functions (depth %d)\n", len(result.AffectedFunctions), result.MaxDepth)
	fmt.Fprintf(os.Stderr, "    Confidence: %d exact, %d heuristic, %d ambiguous skipped, %d unresolved\n",
		result.ResolutionStats.ExactCallEdges,
		result.ResolutionStats.HeuristicCallEdges,
		result.ResolutionStats.AmbiguousCalls,
		result.ResolutionStats.UnresolvedCalls,
	)
	fmt.Fprintf(os.Stderr, "    Risk: ")
	red.Fprintf(os.Stderr, "%d high", len(high))
	fmt.Fprintf(os.Stderr, ", ")
	yellow.Fprintf(os.Stderr, "%d medium", len(medium))
	fmt.Fprintf(os.Stderr, ", ")
	green.Fprintf(os.Stderr, "%d low", len(low))
	fmt.Fprintln(os.Stderr)

	if result.Ownership != nil {
		if oi, ok := result.Ownership.(*ownership.OwnershipInfo); ok && len(oi.SuggestedReviewers) > 0 {
			fmt.Fprintln(os.Stderr)
			bold.Fprintln(os.Stderr, "  Suggested Reviewers:")
			for _, r := range oi.SuggestedReviewers {
				fmt.Fprintf(os.Stderr, "    %s ", r.Owner)
				dim.Fprintf(os.Stderr, "(%d file%s)\n", r.FileCount, pluralS(r.FileCount))
			}
		}
	}

	if len(result.ResolutionIssues) > 0 {
		fmt.Fprintln(os.Stderr)
		bold.Fprintln(os.Stderr, "  Resolution Diagnostics:")
		printResolutionIssues(result.ResolutionIssues, cwd)
	}

	fmt.Fprintln(os.Stderr)
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
		fmt.Fprintf(os.Stderr, "      %s ", f.Node.Name)
		dim.Fprintf(os.Stderr, "(%s:%d) ", relPath, f.Node.StartLine)
		dim.Fprintf(os.Stderr, "[score %d, %d callers, depth %d, %s, %s]", f.ReviewScore, f.CallerCount, f.Depth, formatConfidence(f.Confidence), f.Reason)
		if f.LastAuthor != "" {
			dim.Fprintf(os.Stderr, " by %s", f.LastAuthor)
		}
		fmt.Fprintln(os.Stderr)
		if path := formatImpactPathText(f); path != "" {
			dim.Fprintf(os.Stderr, "        path: %s\n", path)
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
		dim.Fprintf(os.Stderr, "    %s %s %s (%s)\n", issue.Status, issue.Kind, symbol, path)
		if issue.Detail != "" {
			dim.Fprintf(os.Stderr, "      %s\n", issue.Detail)
		}
	}
	if len(issues) > limit {
		dim.Fprintf(os.Stderr, "    ... showing first %d of %d diagnostics\n", limit, len(issues))
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
