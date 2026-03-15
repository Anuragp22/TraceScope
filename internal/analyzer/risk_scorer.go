package analyzer

import (
	"fmt"

	"github.com/anurag/tracescope/internal/graph"
)

// RiskScorer assigns risk levels to affected functions.
type RiskScorer struct{}

// Score assigns a risk level to a function node based on its properties.
// It considers production caller count (excluding test callers) and BFS depth.
func (s *RiskScorer) Score(node *graph.Node, callerCount int, depth int, prodCallerCount int) (RiskLevel, string) {
	// Use production caller count for risk thresholds
	effectiveCallers := prodCallerCount

	// HIGH: 10+ production callers (highly connected)
	if effectiveCallers >= 10 {
		return RiskHigh, fmt.Sprintf("highly connected (%d production callers)", effectiveCallers)
	}

	// HIGH: exported/public function with many production callers
	if node.IsExport && effectiveCallers >= 5 {
		return RiskHigh, fmt.Sprintf("exported function with %d production callers", effectiveCallers)
	}

	// MEDIUM: direct dependency (depth 1) that is exported with some callers
	if depth <= 1 && node.IsExport && effectiveCallers >= 1 {
		return RiskMedium, fmt.Sprintf("direct exported dependency (%d production callers)", effectiveCallers)
	}

	// MEDIUM: 3+ production callers
	if effectiveCallers >= 3 {
		return RiskMedium, fmt.Sprintf("moderately connected (%d production callers)", effectiveCallers)
	}

	if node.IsExport {
		return RiskMedium, "exported/public function"
	}

	// LOW: everything else
	return RiskLow, "internal function with few callers"
}
