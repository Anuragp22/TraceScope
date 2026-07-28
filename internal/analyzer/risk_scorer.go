package analyzer

import (
	"fmt"

	"github.com/anurag/tracescope/internal/graph"
)

// RiskScorer assigns risk levels to affected functions.
// Thresholds default to standard values when zero.
type RiskScorer struct {
	HighCallers         int // callers for HIGH risk (default 10)
	HighExportedCallers int // callers for HIGH when exported (default 5)
	MediumCallers       int // callers for MEDIUM risk (default 3)
}

func (s *RiskScorer) highCallers() int {
	if s.HighCallers > 0 {
		return s.HighCallers
	}
	return 10
}

func (s *RiskScorer) highExportedCallers() int {
	if s.HighExportedCallers > 0 {
		return s.HighExportedCallers
	}
	return 5
}

func (s *RiskScorer) mediumCallers() int {
	if s.MediumCallers > 0 {
		return s.MediumCallers
	}
	return 3
}

// Score assigns a risk level to a function node based on its properties.
// It considers production caller count (excluding test callers) and BFS depth.
func (s *RiskScorer) Score(node *graph.Node, depth int, prodCallerCount int) (RiskLevel, string) {
	effectiveCallers := prodCallerCount

	// HIGH: many production callers (highly connected)
	if effectiveCallers >= s.highCallers() {
		return RiskHigh, fmt.Sprintf("highly connected (%d production callers)", effectiveCallers)
	}

	// HIGH: exported/public function with many production callers
	if node.IsExport && effectiveCallers >= s.highExportedCallers() {
		return RiskHigh, fmt.Sprintf("exported function with %d production callers", effectiveCallers)
	}

	// MEDIUM: direct dependency (depth 1) that is exported with some callers
	if depth <= 1 && node.IsExport && effectiveCallers >= 1 {
		return RiskMedium, fmt.Sprintf("direct exported dependency (%d production callers)", effectiveCallers)
	}

	// MEDIUM: moderate production callers
	if effectiveCallers >= s.mediumCallers() {
		return RiskMedium, fmt.Sprintf("moderately connected (%d production callers)", effectiveCallers)
	}

	// A test with no production callers is not a review priority. This rung has
	// to come before the export fallback below: in Go every TestXxx is an
	// exported name, so without it every test in the blast radius is tagged
	// MEDIUM and crowds the top of the list a reviewer is told to read first.
	// Guarded on zero production callers so it cannot shadow any rule above,
	// all of which require at least one.
	if node.IsTest && effectiveCallers == 0 {
		return RiskLow, "test function with no production callers"
	}

	if node.IsExport {
		return RiskMedium, "exported/public function"
	}

	// LOW: everything else
	return RiskLow, "internal function with few callers"
}
