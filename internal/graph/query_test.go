package graph

import (
	"testing"
)

func TestComputeBlastRadius(t *testing.T) {
	gd := &GraphData{
		Nodes: []Node{
			{ID: "f:main", Type: NodeFunction, Name: "main"},
			{ID: "f:run", Type: NodeFunction, Name: "Run"},
			{ID: "f:helper", Type: NodeFunction, Name: "helper"},
			{ID: "f:utils", Type: NodeFunction, Name: "utils"},
		},
		Edges: []Edge{
			{Source: "f:main", Target: "f:run", Type: EdgeCalls},
			{Source: "f:run", Target: "f:helper", Type: EdgeCalls},
			{Source: "f:utils", Target: "f:helper", Type: EdgeCalls},
		},
	}

	// Change "helper" → blast radius should include "run", "main", "utils"
	result := ComputeBlastRadius(gd, []string{"f:helper"}, 5)

	if len(result.AffectedNodes) != 4 { // helper + run + main + utils
		t.Errorf("expected 4 affected nodes, got %d", len(result.AffectedNodes))
	}

	// "run" and "utils" should be at depth 1
	if result.Depth["f:run"] != 1 {
		t.Errorf("expected depth 1 for 'run', got %d", result.Depth["f:run"])
	}
	if result.Depth["f:utils"] != 1 {
		t.Errorf("expected depth 1 for 'utils', got %d", result.Depth["f:utils"])
	}

	// "main" should be at depth 2
	if result.Depth["f:main"] != 2 {
		t.Errorf("expected depth 2 for 'main', got %d", result.Depth["f:main"])
	}
}

func TestComputeBlastRadius_BoundedDepth(t *testing.T) {
	gd := &GraphData{
		Nodes: []Node{
			{ID: "a", Type: NodeFunction, Name: "a"},
			{ID: "b", Type: NodeFunction, Name: "b"},
			{ID: "c", Type: NodeFunction, Name: "c"},
		},
		Edges: []Edge{
			{Source: "b", Target: "a", Type: EdgeCalls},
			{Source: "c", Target: "b", Type: EdgeCalls},
		},
	}

	// Depth 1 should only reach "b", not "c"
	result := ComputeBlastRadius(gd, []string{"a"}, 1)
	if _, ok := result.AffectedNodes["c"]; ok {
		t.Error("'c' should not be in blast radius with depth 1")
	}
	if _, ok := result.AffectedNodes["b"]; !ok {
		t.Error("'b' should be in blast radius with depth 1")
	}
}

func TestComputeBlastRadius_TracksPathConfidenceAndParent(t *testing.T) {
	gd := &GraphData{
		Nodes: []Node{
			{ID: "seed", Type: NodeFunction, Name: "seed"},
			{ID: "mid", Type: NodeFunction, Name: "mid"},
			{ID: "caller", Type: NodeFunction, Name: "caller"},
		},
		Edges: []Edge{
			{Source: "mid", Target: "seed", Type: EdgeCalls, Confidence: EdgeConfidenceExact},
			{Source: "caller", Target: "mid", Type: EdgeCalls, Confidence: EdgeConfidenceHeuristic},
		},
	}

	result := ComputeBlastRadius(gd, []string{"seed"}, 5)

	if result.Parent["mid"] != "seed" {
		t.Fatalf("expected mid parent seed, got %q", result.Parent["mid"])
	}
	if result.Parent["caller"] != "mid" {
		t.Fatalf("expected caller parent mid, got %q", result.Parent["caller"])
	}
	if result.Confidence["mid"] != EdgeConfidenceExact {
		t.Fatalf("expected exact confidence for mid, got %q", result.Confidence["mid"])
	}
	if result.Confidence["caller"] != EdgeConfidenceHeuristic {
		t.Fatalf("expected heuristic confidence for caller, got %q", result.Confidence["caller"])
	}
}
