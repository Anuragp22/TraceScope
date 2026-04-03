package graph

import "testing"

func TestCompareGraphShape_NormalizesRootRelativePaths(t *testing.T) {
	base := &GraphData{
		Nodes: []Node{
			{ID: "file", Type: NodeFile, Name: "service.go", FilePath: `C:\repo\service.go`},
			{ID: "fn", Type: NodeFunction, Name: "Run", FilePath: `C:\repo\service.go`},
		},
		Edges: []Edge{{Source: "file", Target: "fn", Type: EdgeContains}},
	}
	candidate := &GraphData{
		Nodes: []Node{
			{ID: "file", Type: NodeFile, Name: "service.go", FilePath: "service.go"},
			{ID: "fn", Type: NodeFunction, Name: "Run", FilePath: "service.go"},
		},
		Edges: []Edge{{Source: "file", Target: "fn", Type: EdgeContains}},
	}

	comparison := CompareGraphShape(base, candidate, `C:\repo`, 10)
	if comparison.MissingNodeCount != 0 || comparison.ExtraNodeCount != 0 {
		t.Fatalf("expected no node mismatch, got %+v", comparison)
	}
	if comparison.MissingEdgeCount != 0 || comparison.ExtraEdgeCount != 0 {
		t.Fatalf("expected no edge mismatch, got %+v", comparison)
	}
}
