package output

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/fatih/color"
)

// PrintHotspots outputs the hotspot analysis to the terminal.
func PrintHotspots(result *analyzer.HotspotsResult) {
	cwd, _ := os.Getwd()

	fmt.Fprintln(os.Stderr)
	cyan.Fprintln(os.Stderr, "TraceScope — Hotspot Analysis")
	fmt.Fprintln(os.Stderr)

	if len(result.Hotspots) == 0 {
		dim.Fprintln(os.Stderr, "  No hotspots found.")
		fmt.Fprintln(os.Stderr)
		return
	}

	// Header
	bold.Fprintf(os.Stderr, "  %-4s %-30s %-40s %8s %8s %8s  %s\n",
		"#", "Function", "File", "Inbound", "Outbound", "Coupling", "Risk")
	dim.Fprintf(os.Stderr, "  %s\n", "────────────────────────────────────────────────────────────────────────────────────────────────────────────")

	for i, h := range result.Hotspots {
		relPath := shortPath(h.Node.FilePath, cwd)
		loc := fmt.Sprintf("%s:%d", relPath, h.Node.StartLine)

		riskColor := riskColorFor(h.Risk)

		fmt.Fprintf(os.Stderr, "  %-4d ", i+1)
		riskColor.Fprintf(os.Stderr, "%-30s", truncate(h.Node.Name, 30))
		fmt.Fprintf(os.Stderr, " ")
		dim.Fprintf(os.Stderr, "%-40s", truncate(loc, 40))
		fmt.Fprintf(os.Stderr, " %8d %8d %8d  ", h.InboundCallers, h.OutboundCalls, h.CouplingScore)
		riskColor.Fprintf(os.Stderr, "%s", h.Risk)

		if h.Node.IsExport {
			dim.Fprintf(os.Stderr, " [exported]")
		}
		fmt.Fprintln(os.Stderr)
	}

	fmt.Fprintln(os.Stderr)

	if result.TopN > 0 && result.Total > result.TopN {
		dim.Fprintf(os.Stderr, "  Showing top %d of %d functions with connections.\n", result.TopN, result.Total)
	}

	// Summary
	bold.Fprintln(os.Stderr, "  Summary:")
	fmt.Fprintf(os.Stderr, "    Graph: %d nodes, %d edges\n", result.TotalNodes, result.TotalEdges)
	fmt.Fprintf(os.Stderr, "    Hotspots shown: %d\n", len(result.Hotspots))
	fmt.Fprintln(os.Stderr)
}

func riskColorFor(risk analyzer.RiskLevel) *color.Color {
	switch risk {
	case analyzer.RiskHigh:
		return red
	case analyzer.RiskMedium:
		return yellow
	default:
		return green
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func shortPath(p string, cwd string) string {
	if cwd == "" {
		return p
	}
	rel, err := filepath.Rel(cwd, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}
