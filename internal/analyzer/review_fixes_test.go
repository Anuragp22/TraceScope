package analyzer

import (
	"testing"

	"github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
)

// Bug #2: deleting a file must seed its functions into the blast radius so
// their callers get traversed — a deletion is the highest-impact change.
func TestMapDiffToFunctions_DeletedFile(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "func:1", Type: graph.NodeFunction, Name: "removed1", FilePath: "old.go", StartLine: 1, EndLine: 5},
			{ID: "func:2", Type: graph.NodeFunction, Name: "removed2", FilePath: "old.go", StartLine: 7, EndLine: 12},
		},
	}

	changed := []diff.ChangedFile{
		{Path: "old.go", IsDeleted: true},
	}

	result := MapDiffToFunctions(changed, gd)
	if len(result) != 2 {
		t.Errorf("expected 2 changed functions for a deleted file, got %d", len(result))
	}
}

// Bug #7b: when two graph paths both fuzzy-match the same diff path, the
// chosen match must be deterministic across runs.
func TestMapDiffToFunctions_DeterministicAmbiguousMatch(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "v", Type: graph.NodeFunction, Name: "fromVendor", FilePath: "vendor/utils/helper.go", StartLine: 1, EndLine: 10},
			{ID: "s", Type: graph.NodeFunction, Name: "fromSrc", FilePath: "src/utils/helper.go", StartLine: 1, EndLine: 10},
		},
	}
	changed := []diff.ChangedFile{
		{Path: "utils/helper.go", LineRanges: []diff.LineRange{{Start: 3, End: 5}}},
	}

	var first string
	for i := 0; i < 50; i++ {
		res := MapDiffToFunctions(changed, gd)
		if len(res) != 1 {
			t.Fatalf("expected exactly 1 match, got %d", len(res))
		}
		if first == "" {
			first = res[0].NodeID
			continue
		}
		if res[0].NodeID != first {
			t.Fatalf("ambiguous path match is not deterministic: got %q then %q", first, res[0].NodeID)
		}
	}
}

// Bug #4: a heavily-called function with no outbound calls (a pure sink) must
// rank as a top hotspot, not be buried below low-coupling middle functions.
func TestComputeHotspots_PureSinkRanksHigh(t *testing.T) {
	nodes := []graph.Node{
		{ID: "sink", Type: graph.NodeFunction, Name: "sink", FilePath: "a.go", Language: "go"},
		{ID: "mid", Type: graph.NodeFunction, Name: "mid", FilePath: "b.go", Language: "go"},
		{ID: "upstream", Type: graph.NodeFunction, Name: "upstream", FilePath: "c.go", Language: "go"},
		{ID: "downstream", Type: graph.NodeFunction, Name: "downstream", FilePath: "d.go", Language: "go"},
	}
	edges := []graph.Edge{
		{Source: "upstream", Target: "mid", Type: graph.EdgeCalls},   // mid: in=1
		{Source: "mid", Target: "downstream", Type: graph.EdgeCalls}, // mid: out=1
	}
	// Five callers of the pure sink (sink: in=5, out=0).
	for i := 0; i < 5; i++ {
		id := "caller" + string(rune('0'+i))
		nodes = append(nodes, graph.Node{ID: id, Type: graph.NodeFunction, Name: id, FilePath: "x.go", Language: "go"})
		edges = append(edges, graph.Edge{Source: id, Target: "sink", Type: graph.EdgeCalls})
	}

	gd := &graph.GraphData{Nodes: nodes, Edges: edges}
	result := ComputeHotspots(gd, HotspotsOptions{})

	if len(result.Hotspots) == 0 {
		t.Fatal("expected hotspots")
	}
	if result.Hotspots[0].Node.Name != "sink" {
		t.Errorf("expected pure sink (5 callers) ranked first, got %q", result.Hotspots[0].Node.Name)
	}
}

// Bug #7b: affected functions that tie on every sort key must still come out
// in a deterministic order across runs.
func TestBlastRadius_DeterministicTiedOrder(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "file:t", Type: graph.NodeFile, Name: "t.go", FilePath: "t.go"},
			{ID: "target", Type: graph.NodeFunction, Name: "target", FilePath: "t.go", StartLine: 1, EndLine: 5},
			{ID: "dupA", Type: graph.NodeFunction, Name: "dup", FilePath: "a.go", StartLine: 1, EndLine: 5},
			{ID: "dupB", Type: graph.NodeFunction, Name: "dup", FilePath: "b.go", StartLine: 1, EndLine: 5},
		},
		Edges: []graph.Edge{
			{Source: "file:t", Target: "target", Type: graph.EdgeContains},
			// Two callers with the same name — they tie on every sort key.
			{Source: "dupA", Target: "target", Type: graph.EdgeCalls, Confidence: graph.EdgeConfidenceExact},
			{Source: "dupB", Target: "target", Type: graph.EdgeCalls, Confidence: graph.EdgeConfidenceExact},
		},
	}
	changed := []diff.ChangedFile{
		{Path: "t.go", LineRanges: []diff.LineRange{{Start: 2, End: 3}}},
	}

	var first string
	for i := 0; i < 50; i++ {
		result := NewBlastRadiusAnalyzer(gd, 5, nil).Analyze(changed)
		sig := ""
		for _, af := range result.AffectedFunctions {
			sig += af.Node.ID + ";"
		}
		if first == "" {
			first = sig
			continue
		}
		if sig != first {
			t.Fatalf("affected-function order is not deterministic: got %q then %q", first, sig)
		}
	}
}
