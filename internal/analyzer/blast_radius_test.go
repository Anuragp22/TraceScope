package analyzer

import (
	"testing"

	"github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
)

func TestBlastRadiusAnalyzer_BasicPipeline(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "file:main", Type: graph.NodeFile, Name: "main.go", FilePath: "main.go"},
			{ID: "func:main", Type: graph.NodeFunction, Name: "main", FilePath: "main.go", StartLine: 3, EndLine: 10},
			{ID: "func:run", Type: graph.NodeFunction, Name: "Run", FilePath: "main.go", StartLine: 12, EndLine: 20, IsExport: true},
			{ID: "func:caller", Type: graph.NodeFunction, Name: "caller", FilePath: "other.go", StartLine: 1, EndLine: 5},
			{ID: "file:other", Type: graph.NodeFile, Name: "other.go", FilePath: "other.go"},
		},
		Edges: []graph.Edge{
			{Source: "file:main", Target: "func:main", Type: graph.EdgeContains},
			{Source: "file:main", Target: "func:run", Type: graph.EdgeContains},
			{Source: "file:other", Target: "func:caller", Type: graph.EdgeContains},
			{Source: "func:caller", Target: "func:run", Type: graph.EdgeCalls},
		},
	}

	changedFiles := []diff.ChangedFile{
		{Path: "main.go", LineRanges: []diff.LineRange{{Start: 14, End: 16}}},
	}

	ba := NewBlastRadiusAnalyzer(gd, 5, nil)
	result := ba.Analyze(changedFiles)

	if len(result.ChangedFunctions) != 1 {
		t.Fatalf("expected 1 changed function, got %d", len(result.ChangedFunctions))
	}
	if result.ChangedFunctions[0].Node.Name != "Run" {
		t.Errorf("expected 'Run' as changed function, got %q", result.ChangedFunctions[0].Node.Name)
	}

	// "caller" should be in blast radius (it calls Run)
	found := false
	for _, af := range result.AffectedFunctions {
		if af.Node.Name == "caller" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'caller' in blast radius")
	}
}

func TestBlastRadiusAnalyzer_SeedDedup(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "file:a", Type: graph.NodeFile, Name: "a.go", FilePath: "a.go"},
			{ID: "func:a1", Type: graph.NodeFunction, Name: "a1", FilePath: "a.go", StartLine: 1, EndLine: 5},
		},
		Edges: []graph.Edge{
			{Source: "file:a", Target: "func:a1", Type: graph.EdgeContains},
		},
	}

	changedFiles := []diff.ChangedFile{
		{Path: "a.go", LineRanges: []diff.LineRange{{Start: 2, End: 3}}},
	}

	ba := NewBlastRadiusAnalyzer(gd, 5, nil)
	result := ba.Analyze(changedFiles)

	// func:a1 is a seed, file:a should NOT be added as seed since func was found
	if len(result.ChangedFunctions) != 1 {
		t.Errorf("expected 1 changed function, got %d", len(result.ChangedFunctions))
	}
}

func TestBlastRadiusAnalyzer_DeterministicOutput(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "file:a", Type: graph.NodeFile, Name: "a.go", FilePath: "a.go"},
			{ID: "func:target", Type: graph.NodeFunction, Name: "target", FilePath: "a.go", StartLine: 1, EndLine: 5},
			{ID: "func:beta", Type: graph.NodeFunction, Name: "beta", FilePath: "b.go", StartLine: 1, EndLine: 5, IsExport: true},
			{ID: "func:alpha", Type: graph.NodeFunction, Name: "alpha", FilePath: "c.go", StartLine: 1, EndLine: 5, IsExport: true},
		},
		Edges: []graph.Edge{
			{Source: "file:a", Target: "func:target", Type: graph.EdgeContains},
			{Source: "func:beta", Target: "func:target", Type: graph.EdgeCalls},
			{Source: "func:alpha", Target: "func:target", Type: graph.EdgeCalls},
		},
	}

	changedFiles := []diff.ChangedFile{
		{Path: "a.go", LineRanges: []diff.LineRange{{Start: 2, End: 3}}},
	}

	// Run twice and check order is same
	ba := NewBlastRadiusAnalyzer(gd, 5, nil)
	r1 := ba.Analyze(changedFiles)
	r2 := ba.Analyze(changedFiles)

	if len(r1.AffectedFunctions) != len(r2.AffectedFunctions) {
		t.Fatal("non-deterministic result count")
	}
	for i := range r1.AffectedFunctions {
		if r1.AffectedFunctions[i].Node.Name != r2.AffectedFunctions[i].Node.Name {
			t.Errorf("non-deterministic order at index %d: %q vs %q",
				i, r1.AffectedFunctions[i].Node.Name, r2.AffectedFunctions[i].Node.Name)
		}
	}
}
