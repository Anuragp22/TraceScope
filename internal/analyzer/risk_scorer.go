package analyzer

import (
	"github.com/anurag/tracescope/internal/graph"
)

// RiskScorer assigns risk levels to affected functions.
type RiskScorer struct{}

// Score assigns a risk level to a function node based on its properties.
func (s *RiskScorer) Score(node *graph.Node, callerCount int) (RiskLevel, string) {
	// HIGH: 10+ callers (highly connected) or is a test file function with many callers
	if callerCount >= 10 {
		return RiskHigh, "highly connected (10+ callers)"
	}

	// HIGH: exported/public function with many callers
	if node.IsExport && callerCount >= 5 {
		return RiskHigh, "exported function with 5+ callers"
	}

	// MEDIUM: 3-10 callers or is exported/public
	if callerCount >= 3 {
		return RiskMedium, "moderately connected (3+ callers)"
	}

	if node.IsExport {
		return RiskMedium, "exported/public function"
	}

	// LOW: everything else
	return RiskLow, "internal function with few callers"
}
