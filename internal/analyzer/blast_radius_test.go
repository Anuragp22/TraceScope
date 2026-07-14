package analyzer

import (
	"fmt"
	"testing"

	"github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
)

// TestBlastRadiusAnalyzer_ReviewScoreDoesNotSaturateOnCallerCount asserts that
// two same-tier affected functions differing only in production caller count get
// strictly different review scores. The old scoring capped the caller bonus, so a
// 10-caller and a 60-caller HIGH function tied — the most impactful cases were
// compressed away. The score must separate them.
func TestBlastRadiusAnalyzer_ReviewScoreDoesNotSaturateOnCallerCount(t *testing.T) {
	nodes := []graph.Node{
		{ID: "file:t", Type: graph.NodeFile, Name: "t.go", FilePath: "t.go"},
		{ID: "func:target", Type: graph.NodeFunction, Name: "target", FilePath: "t.go", StartLine: 1, EndLine: 3},
		{ID: "func:hubSmall", Type: graph.NodeFunction, Name: "hubSmall", FilePath: "s.go", StartLine: 1, EndLine: 3},
		{ID: "func:hubLarge", Type: graph.NodeFunction, Name: "hubLarge", FilePath: "l.go", StartLine: 1, EndLine: 3},
	}
	edges := []graph.Edge{
		{Source: "file:t", Target: "func:target", Type: graph.EdgeContains},
		{Source: "func:hubSmall", Target: "func:target", Type: graph.EdgeCalls},
		{Source: "func:hubLarge", Target: "func:target", Type: graph.EdgeCalls},
	}
	addCallers := func(hub string, n int) {
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("func:%s_c%d", hub, i)
			nodes = append(nodes, graph.Node{ID: id, Type: graph.NodeFunction, Name: id, FilePath: hub + ".go", StartLine: i + 10, EndLine: i + 11})
			edges = append(edges, graph.Edge{Source: id, Target: "func:" + hub, Type: graph.EdgeCalls})
		}
	}
	addCallers("hubSmall", 10) // HIGH (>= 10 prod callers)
	addCallers("hubLarge", 60) // HIGH, but far more connected

	gd := &graph.GraphData{Nodes: nodes, Edges: edges}
	result := NewBlastRadiusAnalyzer(gd, 5, nil).Analyze([]diff.ChangedFile{
		{Path: "t.go", LineRanges: []diff.LineRange{{Start: 2, End: 2}}},
	})

	var small, large *AffectedFunction
	for i := range result.AffectedFunctions {
		switch result.AffectedFunctions[i].Node.Name {
		case "hubSmall":
			small = &result.AffectedFunctions[i]
		case "hubLarge":
			large = &result.AffectedFunctions[i]
		}
	}
	if small == nil || large == nil {
		t.Fatalf("expected both hubs in blast radius; got %d affected", len(result.AffectedFunctions))
	}
	if small.Risk != RiskHigh || large.Risk != RiskHigh {
		t.Fatalf("expected both HIGH; got small=%s large=%s", small.Risk, large.Risk)
	}
	if large.ReviewScore <= small.ReviewScore {
		t.Fatalf("caller-count saturation: hubLarge (60 callers) score %d should exceed hubSmall (10 callers) score %d",
			large.ReviewScore, small.ReviewScore)
	}
}

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
			if af.Confidence != graph.EdgeConfidenceExact {
				t.Fatalf("expected caller confidence EXACT, got %q", af.Confidence)
			}
			if len(af.ImpactPath) != 2 {
				t.Fatalf("expected 2-step impact path, got %d", len(af.ImpactPath))
			}
			if af.ImpactPath[0].Node.Name != "Run" || af.ImpactPath[1].Node.Name != "caller" {
				t.Fatalf("unexpected impact path: %v", af.ImpactPath)
			}
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

func TestBlastRadiusAnalyzer_ReviewScorePrioritizesDirectExportedProdImpact(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "file:target", Type: graph.NodeFile, Name: "target.go", FilePath: "target.go"},
			{ID: "func:target", Type: graph.NodeFunction, Name: "target", FilePath: "target.go", StartLine: 1, EndLine: 3, IsExport: true},
			{ID: "func:direct", Type: graph.NodeFunction, Name: "directCaller", FilePath: "prod.go", StartLine: 1, EndLine: 5, IsExport: true},
			{ID: "func:test", Type: graph.NodeFunction, Name: "testCaller", FilePath: "prod_test.go", StartLine: 1, EndLine: 5, IsTest: true},
		},
		Edges: []graph.Edge{
			{Source: "file:target", Target: "func:target", Type: graph.EdgeContains},
			{Source: "func:direct", Target: "func:target", Type: graph.EdgeCalls, Confidence: graph.EdgeConfidenceExact},
			{Source: "func:test", Target: "func:target", Type: graph.EdgeCalls, Confidence: graph.EdgeConfidenceExact},
		},
		ResolutionIssues: []graph.ResolutionIssue{
			{Kind: "call", Status: "ambiguous", FilePath: "prod.go", Line: 3, Symbol: "run"},
		},
	}

	result := NewBlastRadiusAnalyzer(gd, 5, nil).Analyze([]diff.ChangedFile{
		{Path: "target.go", LineRanges: []diff.LineRange{{Start: 2, End: 2}}},
	})

	if len(result.AffectedFunctions) != 2 {
		t.Fatalf("expected 2 affected functions, got %d", len(result.AffectedFunctions))
	}
	if result.AffectedFunctions[0].Node.Name != "directCaller" {
		t.Fatalf("expected directCaller to rank first, got %s", result.AffectedFunctions[0].Node.Name)
	}
	if result.AffectedFunctions[0].ReviewScore <= result.AffectedFunctions[1].ReviewScore {
		t.Fatalf("expected directCaller score %d to exceed %d", result.AffectedFunctions[0].ReviewScore, result.AffectedFunctions[1].ReviewScore)
	}
	if len(result.ResolutionIssues) != 1 || result.ResolutionIssues[0].Symbol != "run" {
		t.Fatalf("expected resolution diagnostics to propagate, got %+v", result.ResolutionIssues)
	}
}
