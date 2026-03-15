package output

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/anurag/tracescope/internal/analyzer"
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
	// Cache cwd once instead of calling os.Getwd() per function
	cwd, _ := os.Getwd()

	fmt.Fprintln(os.Stderr)
	cyan.Fprintln(os.Stderr, "TraceScope — Blast Radius Analysis")
	fmt.Fprintln(os.Stderr)

	// Changed files
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

	// Changed functions
	bold.Fprintf(os.Stderr, "  Changed Functions (%d):\n", len(result.ChangedFunctions))
	for _, cf := range result.ChangedFunctions {
		relPath := shortPathCwd(cf.FilePath, cwd)
		fmt.Fprintf(os.Stderr, "    %s ", cf.Node.Name)
		dim.Fprintf(os.Stderr, "(%s:%d)\n", relPath, cf.Node.StartLine)
	}
	fmt.Fprintln(os.Stderr)

	// Blast radius by risk
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

	// Summary
	bold.Fprintln(os.Stderr, "  Summary:")
	fmt.Fprintf(os.Stderr, "    Graph: %d nodes, %d edges\n", result.TotalNodes, result.TotalEdges)
	fmt.Fprintf(os.Stderr, "    Changed: %d files, %d functions\n", len(result.ChangedFiles), len(result.ChangedFunctions))
	fmt.Fprintf(os.Stderr, "    Blast radius: %d affected functions (depth %d)\n", len(result.AffectedFunctions), result.MaxDepth)
	fmt.Fprintf(os.Stderr, "    Risk: ")
	red.Fprintf(os.Stderr, "%d high", len(high))
	fmt.Fprintf(os.Stderr, ", ")
	yellow.Fprintf(os.Stderr, "%d medium", len(medium))
	fmt.Fprintf(os.Stderr, ", ")
	green.Fprintf(os.Stderr, "%d low", len(low))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr)
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

	// Sort each group by caller count descending, then name for stability
	sortGroup := func(s []analyzer.AffectedFunction) {
		sort.Slice(s, func(i, j int) bool {
			if s[i].CallerCount != s[j].CallerCount {
				return s[i].CallerCount > s[j].CallerCount
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
		relPath := shortPathCwd(f.Node.FilePath, cwd)
		fmt.Fprintf(os.Stderr, "      %s ", f.Node.Name)
		dim.Fprintf(os.Stderr, "(%s:%d) ", relPath, f.Node.StartLine)
		dim.Fprintf(os.Stderr, "[%d callers, depth %d — %s]\n", f.CallerCount, f.Depth, f.Reason)
	}
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
