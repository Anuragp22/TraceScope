package output

import (
	"fmt"
	"os"

	"github.com/anurag/tracescope/internal/graph"
)

// PrintPath outputs a call path to the terminal.
func PrintPath(result *graph.PathResult) {
	cwd, _ := os.Getwd()

	fmt.Fprintln(reportOut)

	if !result.Found {
		cyan.Fprintln(reportOut, "TraceScope — Call Path")
		fmt.Fprintln(reportOut)
		yellow.Fprintf(reportOut, "  %s\n", result.Message)
		fmt.Fprintln(reportOut)

		if result.Source != nil {
			dim.Fprintf(reportOut, "  Source: %s (%s:%d)\n",
				result.Source.Name, shortPath(result.Source.FilePath, cwd), result.Source.StartLine)
		}
		if result.Target != nil {
			dim.Fprintf(reportOut, "  Target: %s (%s:%d)\n",
				result.Target.Name, shortPath(result.Target.FilePath, cwd), result.Target.StartLine)
		}
		fmt.Fprintln(reportOut)
		return
	}

	cyan.Fprintf(reportOut, "TraceScope — Call Path")
	if result.Length == 0 {
		fmt.Fprintf(reportOut, " (same function)\n")
	} else {
		fmt.Fprintf(reportOut, " (%d hop%s)\n", result.Length, plural(result.Length))
	}
	fmt.Fprintln(reportOut)

	for i, step := range result.Path {
		relPath := shortPath(step.Node.FilePath, cwd)

		// Function name
		fmt.Fprintf(reportOut, "  ")
		bold.Fprintf(reportOut, "%-30s ", step.Node.Name)
		dim.Fprintf(reportOut, "%s:%d", relPath, step.Node.StartLine)
		if step.Node.Package != "" {
			dim.Fprintf(reportOut, " [%s]", step.Node.Package)
		}
		fmt.Fprintln(reportOut)

		// Arrow to next step
		if i < len(result.Path)-1 {
			dim.Fprintf(reportOut, "    │\n")
			dim.Fprintf(reportOut, "    │ %s\n", step.EdgeType)
			dim.Fprintf(reportOut, "    ▼\n")
		}
	}

	fmt.Fprintln(reportOut)
}

// PrintNodeMatches shows ambiguous matches for the user to disambiguate.
// This is diagnostic, not report output: it accompanies an error, so it goes to
// stderr and never pollutes a redirected report.
func PrintNodeMatches(query string, matches []graph.NodeMatch) {
	cwd, _ := os.Getwd()

	fmt.Fprintln(os.Stderr)
	yellow.Fprintf(os.Stderr, "  Ambiguous: %q matches %d functions:\n\n", query, len(matches))

	for i, m := range matches {
		relPath := shortPath(m.Node.FilePath, cwd)
		fmt.Fprintf(os.Stderr, "    %d. ", i+1)
		bold.Fprintf(os.Stderr, "%s", m.Qualifier)
		dim.Fprintf(os.Stderr, "  (%s:%d)", relPath, m.Node.StartLine)
		dim.Fprintf(os.Stderr, "  [%s]", m.MatchType)
		fmt.Fprintln(os.Stderr)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  Use a more specific name, e.g.: tracescope why graph.Build analyzer.Score")
	fmt.Fprintln(os.Stderr)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
