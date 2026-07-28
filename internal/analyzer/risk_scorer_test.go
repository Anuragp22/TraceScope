package analyzer

import (
	"testing"

	"github.com/anurag/tracescope/internal/graph"
)

func TestRiskScorer_High(t *testing.T) {
	scorer := &RiskScorer{}

	// 10+ production callers → HIGH
	node := &graph.Node{Name: "helper", IsExport: false}
	risk, _ := scorer.Score(node, 2, 15)
	if risk != RiskHigh {
		t.Errorf("expected HIGH for 15 callers, got %s", risk)
	}

	// Exported + 5+ production callers → HIGH
	node = &graph.Node{Name: "GetUser", IsExport: true}
	risk, _ = scorer.Score(node, 2, 7)
	if risk != RiskHigh {
		t.Errorf("expected HIGH for exported with 7 callers, got %s", risk)
	}
}

func TestRiskScorer_Medium(t *testing.T) {
	scorer := &RiskScorer{}

	// 3+ production callers → MEDIUM
	node := &graph.Node{Name: "process", IsExport: false}
	risk, _ := scorer.Score(node, 2, 4)
	if risk != RiskMedium {
		t.Errorf("expected MEDIUM for 4 callers, got %s", risk)
	}

	// Exported with few callers → MEDIUM
	node = &graph.Node{Name: "Run", IsExport: true}
	risk, _ = scorer.Score(node, 2, 1)
	if risk != RiskMedium {
		t.Errorf("expected MEDIUM for exported with 1 caller, got %s", risk)
	}
}

func TestRiskScorer_Low(t *testing.T) {
	scorer := &RiskScorer{}

	node := &graph.Node{Name: "doStuff", IsExport: false}
	risk, _ := scorer.Score(node, 3, 1)
	if risk != RiskLow {
		t.Errorf("expected LOW for internal with 1 caller, got %s", risk)
	}
}

func TestRiskScorer_DepthAwareness(t *testing.T) {
	scorer := &RiskScorer{}

	// Depth 1, exported, 1 production caller → MEDIUM (direct dependency)
	node := &graph.Node{Name: "Handler", IsExport: true}
	risk, _ := scorer.Score(node, 1, 1)
	if risk != RiskMedium {
		t.Errorf("expected MEDIUM for direct exported dependency, got %s", risk)
	}
}

// TestComputeReviewScore_NoExportDoubleCount asserts that export status does not
// add a bonus on top of the risk tier. Exportedness already helps decide the tier
// (via RiskScorer.Score); re-adding it in the review score double-counts the same
// evidence. Two nodes with the same tier and caller count, differing only in
// IsExport, must score identically.
func TestComputeReviewScore_NoExportDoubleCount(t *testing.T) {
	plain := &graph.Node{Name: "a", IsExport: false}
	exported := &graph.Node{Name: "b", IsExport: true}
	s1 := computeReviewScore(plain, RiskHigh, graph.EdgeConfidenceExact, 5, 1)
	s2 := computeReviewScore(exported, RiskHigh, graph.EdgeConfidenceExact, 5, 1)
	if s1 != s2 {
		t.Fatalf("export double-counted: plain=%d exported=%d (tier already reflects export)", s1, s2)
	}
}

// TestRiskScorer_TestFunctionNotMedium pins the rung that keeps test functions
// out of the top of the review list. Every Go TestXxx is an exported name, so
// before this rung existed the "exported/public function" fallback tagged every
// test in the blast radius MEDIUM — and on a change whose only direct callers
// are tests, the entire top of the ranking was test code.
func TestRiskScorer_TestFunctionNotMedium(t *testing.T) {
	scorer := &RiskScorer{}

	test := &graph.Node{Name: "TestThing", IsExport: true, IsTest: true}
	if risk, _ := scorer.Score(test, 1, 0); risk != RiskLow {
		t.Errorf("expected LOW for a test function with no production callers, got %s", risk)
	}

	// The rung must not shadow a genuinely connected node that happens to live
	// in a test file — production callers still decide the tier.
	helper := &graph.Node{Name: "Helper", IsExport: true, IsTest: true}
	if risk, _ := scorer.Score(helper, 1, 12); risk != RiskHigh {
		t.Errorf("expected HIGH for a test-file node with 12 production callers, got %s", risk)
	}

	// A non-test exported function with no callers is still MEDIUM — the rung
	// must key on IsTest, not on the caller count alone.
	prod := &graph.Node{Name: "Exported", IsExport: true}
	if risk, _ := scorer.Score(prod, 1, 0); risk != RiskMedium {
		t.Errorf("expected MEDIUM for an exported non-test function, got %s", risk)
	}
}

func TestRiskScorer_TestCallersFiltered(t *testing.T) {
	scorer := &RiskScorer{}

	// Score sees only production callers (test callers are filtered upstream in
	// buildCallerCountMaps). With 2 production callers → not HIGH.
	node := &graph.Node{Name: "helper", IsExport: false}
	risk, _ := scorer.Score(node, 2, 2)
	if risk != RiskLow {
		t.Errorf("expected LOW when production callers are low, got %s", risk)
	}
}
