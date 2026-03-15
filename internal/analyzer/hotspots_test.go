package analyzer

import (
	"testing"

	"github.com/anurag/tracescope/internal/graph"
)

func TestComputeHotspots_Basic(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "f1", Type: graph.NodeFunction, Name: "hub", FilePath: "a.go", Language: "go", IsExport: true},
			{ID: "f2", Type: graph.NodeFunction, Name: "caller1", FilePath: "b.go", Language: "go"},
			{ID: "f3", Type: graph.NodeFunction, Name: "caller2", FilePath: "c.go", Language: "go"},
			{ID: "f4", Type: graph.NodeFunction, Name: "leaf", FilePath: "d.go", Language: "go"},
		},
		Edges: []graph.Edge{
			{Source: "f2", Target: "f1", Type: graph.EdgeCalls},
			{Source: "f3", Target: "f1", Type: graph.EdgeCalls},
			{Source: "f1", Target: "f4", Type: graph.EdgeCalls},
		},
	}

	result := ComputeHotspots(gd, HotspotsOptions{})

	if len(result.Hotspots) == 0 {
		t.Fatal("expected hotspots")
	}

	// "hub" has 2 inbound, 1 outbound → coupling 2
	// Should be first
	if result.Hotspots[0].Node.Name != "hub" {
		t.Errorf("expected 'hub' as top hotspot, got %q", result.Hotspots[0].Node.Name)
	}
	if result.Hotspots[0].InboundCallers != 2 {
		t.Errorf("expected 2 inbound callers, got %d", result.Hotspots[0].InboundCallers)
	}
	if result.Hotspots[0].OutboundCalls != 1 {
		t.Errorf("expected 1 outbound call, got %d", result.Hotspots[0].OutboundCalls)
	}
	if result.Hotspots[0].CouplingScore != 2 {
		t.Errorf("expected coupling score 2, got %d", result.Hotspots[0].CouplingScore)
	}
}

func TestComputeHotspots_TopN(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "f1", Type: graph.NodeFunction, Name: "a", FilePath: "a.go", Language: "go"},
			{ID: "f2", Type: graph.NodeFunction, Name: "b", FilePath: "b.go", Language: "go"},
			{ID: "f3", Type: graph.NodeFunction, Name: "c", FilePath: "c.go", Language: "go"},
		},
		Edges: []graph.Edge{
			{Source: "f1", Target: "f2", Type: graph.EdgeCalls},
			{Source: "f2", Target: "f3", Type: graph.EdgeCalls},
		},
	}

	result := ComputeHotspots(gd, HotspotsOptions{TopN: 1})
	if len(result.Hotspots) != 1 {
		t.Errorf("expected 1 hotspot with top 1, got %d", len(result.Hotspots))
	}
	if result.Total == 0 {
		t.Error("expected Total to be set when TopN truncates")
	}
}

func TestComputeHotspots_LanguageFilter(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "f1", Type: graph.NodeFunction, Name: "goFunc", FilePath: "a.go", Language: "go"},
			{ID: "f2", Type: graph.NodeFunction, Name: "jsFunc", FilePath: "b.js", Language: "javascript"},
		},
		Edges: []graph.Edge{
			{Source: "f1", Target: "f2", Type: graph.EdgeCalls},
		},
	}

	result := ComputeHotspots(gd, HotspotsOptions{Languages: map[string]bool{"go": true}})
	for _, h := range result.Hotspots {
		if h.Node.Language != "go" {
			t.Errorf("expected only go functions, got %q", h.Node.Language)
		}
	}
}

func TestComputeHotspots_Empty(t *testing.T) {
	gd := &graph.GraphData{}
	result := ComputeHotspots(gd, HotspotsOptions{})
	if len(result.Hotspots) != 0 {
		t.Errorf("expected 0 hotspots for empty graph, got %d", len(result.Hotspots))
	}
}

func TestComputeHotspots_RiskClassification(t *testing.T) {
	nodes := []graph.Node{
		{ID: "target", Type: graph.NodeFunction, Name: "target", FilePath: "a.go", Language: "go"},
	}
	// Create 10 callers
	edges := make([]graph.Edge, 10)
	for i := 0; i < 10; i++ {
		callerID := "c" + string(rune('0'+i))
		nodes = append(nodes, graph.Node{ID: callerID, Type: graph.NodeFunction, Name: "caller", FilePath: "b.go", Language: "go"})
		edges[i] = graph.Edge{Source: callerID, Target: "target", Type: graph.EdgeCalls}
	}

	gd := &graph.GraphData{Nodes: nodes, Edges: edges}
	result := ComputeHotspots(gd, HotspotsOptions{})

	found := false
	for _, h := range result.Hotspots {
		if h.Node.Name == "target" {
			found = true
			if h.Risk != RiskHigh {
				t.Errorf("expected HIGH risk for 10 callers, got %s", h.Risk)
			}
		}
	}
	if !found {
		t.Error("expected to find 'target' in hotspots")
	}
}
