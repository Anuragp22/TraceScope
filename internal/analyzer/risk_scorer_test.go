package analyzer

import (
	"testing"

	"github.com/anurag/tracescope/internal/graph"
)

func TestRiskScorer_High(t *testing.T) {
	scorer := &RiskScorer{}

	// 10+ production callers → HIGH
	node := &graph.Node{Name: "helper", IsExport: false}
	risk, _ := scorer.Score(node, 15, 2, 15)
	if risk != RiskHigh {
		t.Errorf("expected HIGH for 15 callers, got %s", risk)
	}

	// Exported + 5+ production callers → HIGH
	node = &graph.Node{Name: "GetUser", IsExport: true}
	risk, _ = scorer.Score(node, 7, 2, 7)
	if risk != RiskHigh {
		t.Errorf("expected HIGH for exported with 7 callers, got %s", risk)
	}
}

func TestRiskScorer_Medium(t *testing.T) {
	scorer := &RiskScorer{}

	// 3+ production callers → MEDIUM
	node := &graph.Node{Name: "process", IsExport: false}
	risk, _ := scorer.Score(node, 4, 2, 4)
	if risk != RiskMedium {
		t.Errorf("expected MEDIUM for 4 callers, got %s", risk)
	}

	// Exported with few callers → MEDIUM
	node = &graph.Node{Name: "Run", IsExport: true}
	risk, _ = scorer.Score(node, 1, 2, 1)
	if risk != RiskMedium {
		t.Errorf("expected MEDIUM for exported with 1 caller, got %s", risk)
	}
}

func TestRiskScorer_Low(t *testing.T) {
	scorer := &RiskScorer{}

	node := &graph.Node{Name: "doStuff", IsExport: false}
	risk, _ := scorer.Score(node, 1, 3, 1)
	if risk != RiskLow {
		t.Errorf("expected LOW for internal with 1 caller, got %s", risk)
	}
}

func TestRiskScorer_DepthAwareness(t *testing.T) {
	scorer := &RiskScorer{}

	// Depth 1, exported, 1 production caller → MEDIUM (direct dependency)
	node := &graph.Node{Name: "Handler", IsExport: true}
	risk, _ := scorer.Score(node, 1, 1, 1)
	if risk != RiskMedium {
		t.Errorf("expected MEDIUM for direct exported dependency, got %s", risk)
	}
}

func TestRiskScorer_TestCallersFiltered(t *testing.T) {
	scorer := &RiskScorer{}

	// 15 total callers but only 2 production callers → not HIGH
	node := &graph.Node{Name: "helper", IsExport: false}
	risk, _ := scorer.Score(node, 15, 2, 2)
	if risk != RiskLow {
		t.Errorf("expected LOW when production callers are low, got %s", risk)
	}
}
