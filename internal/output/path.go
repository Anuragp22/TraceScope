package output

import (
	"fmt"
	"os"

	"github.com/anurag/tracescope/internal/graph"
)

// PrintPath outputs a call path to the terminal.
func PrintPath(result *graph.PathResult) {
	cwd, _ := os.Getwd()

	fmt.Fprintln(os.Stderr)

	if !result.Found {
		cyan.Fprintln(os.Stderr, "TraceScope — Call Path")
		fmt.Fprintln(os.Stderr)
		yellow.Fprintf(os.Stderr, "  %s\n", result.Message)
		fmt.Fprintln(os.Stderr)

		if result.Source != nil {
			dim.Fprintf(os.Stderr, "  Source: %s (%s:%d)\n",
				result.Source.Name, shortPath(result.Source.FilePath, cwd), result.Source.StartLine)
		}
		if result.Target != nil {
			dim.Fprintf(os.Stderr, "  Target: %s (%s:%d)\n",
				result.Target.Name, shortPath(result.Target.FilePath, cwd), result.Target.StartLine)
		}
		fmt.Fprintln(os.Stderr)
		return
	}

	cyan.Fprintf(os.Stderr, "TraceScope — Call Path")
	if result.Length == 0 {
		fmt.Fprintf(os.Stderr, " (same function)\n")
	} else {
		fmt.Fprintf(os.Stderr, " (%d hop%s)\n", result.Length, plural(result.Length))
	}
	fmt.Fprintln(os.Stderr)

	for i, step := range result.Path {
		relPath := shortPath(step.Node.FilePath, cwd)

		// Function name
		fmt.Fprintf(os.Stderr, "  ")
		bold.Fprintf(os.Stderr, "%-30s ", step.Node.Name)
		dim.Fprintf(os.Stderr, "%s:%d", relPath, step.Node.StartLine)
		if step.Node.Package != "" {
			dim.Fprintf(os.Stderr, " [%s]", step.Node.Package)
		}
		fmt.Fprintln(os.Stderr)

		// Arrow to next step
		if i < len(result.Path)-1 {
			dim.Fprintf(os.Stderr, "    │\n")
			dim.Fprintf(os.Stderr, "    │ %s\n", step.EdgeType)
			dim.Fprintf(os.Stderr, "    ▼\n")
		}
	}

	fmt.Fprintln(os.Stderr)
}

// PrintNodeMatches shows ambiguous matches for the user to disambiguate.
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
