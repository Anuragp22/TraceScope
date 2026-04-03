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

func TestCompareGraphShape_IgnoresSyntheticDefaultFunctionAliases(t *testing.T) {
	base := &GraphData{
		Nodes: []Node{
			{ID: "file", Type: NodeFile, Name: "page.tsx", FilePath: "web/app/page.tsx"},
			{ID: "default", Type: NodeFunction, Name: "default", FilePath: "web/app/page.tsx", StartLine: 1},
			{ID: "page", Type: NodeFunction, Name: "DashboardPage", FilePath: "web/app/page.tsx", StartLine: 1},
		},
		Edges: []Edge{
			{Source: "file", Target: "default", Type: EdgeContains},
			{Source: "file", Target: "page", Type: EdgeContains},
		},
	}
	candidate := &GraphData{
		Nodes: []Node{
			{ID: "file", Type: NodeFile, Name: "page.tsx", FilePath: "web/app/page.tsx"},
			{ID: "page", Type: NodeFunction, Name: "DashboardPage", FilePath: "web/app/page.tsx", StartLine: 1},
		},
		Edges: []Edge{{Source: "file", Target: "page", Type: EdgeContains}},
	}

	comparison := CompareGraphShape(base, candidate, "", 10)
	if comparison.MissingNodeCount != 0 || comparison.ExtraNodeCount != 0 {
		t.Fatalf("expected synthetic default alias to be ignored, got %+v", comparison)
	}
	if comparison.MissingEdgeCount != 0 || comparison.ExtraEdgeCount != 0 {
		t.Fatalf("expected synthetic default alias edges to be ignored, got %+v", comparison)
	}
}

func TestCompareGraphShape_NormalizesGoImportRepresentativeFiles(t *testing.T) {
	base := &GraphData{
		Nodes: []Node{
			{ID: "main", Type: NodeFile, Name: "main.go", FilePath: "cmd/tracescope/main.go", Language: "go"},
			{ID: "index", Type: NodeFile, Name: "index.go", FilePath: "internal/cmd/index.go", Language: "go"},
		},
		Edges: []Edge{{Source: "main", Target: "index", Type: EdgeImports}},
	}
	candidate := &GraphData{
		Nodes: []Node{
			{ID: "main", Type: NodeFile, Name: "main.go", FilePath: "cmd/tracescope/main.go", Language: "go"},
			{ID: "root", Type: NodeFile, Name: "root.go", FilePath: "internal/cmd/root.go", Language: "go"},
		},
		Edges: []Edge{{Source: "main", Target: "root", Type: EdgeImports}},
	}

	comparison := CompareGraphShape(base, candidate, "", 10)
	if comparison.MissingEdgeCount != 0 || comparison.ExtraEdgeCount != 0 {
		t.Fatalf("expected Go package import representatives to compare equal, got %+v", comparison)
	}
}
