package graph

import (
	"testing"

	"github.com/anurag/tracescope/internal/parser"
)

func TestBuilder_BasicGraph(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "main.go",
			Language: parser.LangGo,
			Package:  "main",
			Functions: []parser.FunctionDef{
				{Name: "main", StartLine: 5, EndLine: 10, IsExport: false},
			},
			Calls: []parser.FunctionCall{
				{Name: "Run", Receiver: "", Line: 7},
			},
		},
		{
			FilePath: "server.go",
			Language: parser.LangGo,
			Package:  "main",
			Functions: []parser.FunctionDef{
				{Name: "Run", StartLine: 3, EndLine: 15, IsExport: true},
			},
		},
	}

	b := NewBuilder()
	gd := b.Build(results)

	// Should have 2 file nodes + 2 function nodes = 4
	if len(gd.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(gd.Nodes))
	}

	// Should have CONTAINS edges (2) + CALLS edge (1) = at least 3
	containsCount := 0
	callsCount := 0
	for _, e := range gd.Edges {
		switch e.Type {
		case EdgeContains:
			containsCount++
		case EdgeCalls:
			callsCount++
		}
	}
	if containsCount != 2 {
		t.Errorf("expected 2 CONTAINS edges, got %d", containsCount)
	}
	if callsCount != 1 {
		t.Errorf("expected 1 CALLS edge, got %d", callsCount)
	}
}

func TestBuilder_ClassNodes(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "models.go",
			Language: parser.LangGo,
			Package:  "models",
			Classes: []parser.ClassDef{
				{Name: "User", StartLine: 3, EndLine: 8, Kind: "struct", IsExport: true},
				{Name: "Service", StartLine: 10, EndLine: 20, Kind: "interface", IsExport: true},
			},
		},
	}

	b := NewBuilder()
	gd := b.Build(results)

	classCount := 0
	for _, n := range gd.Nodes {
		if n.Type == NodeClass {
			classCount++
		}
	}
	if classCount != 2 {
		t.Errorf("expected 2 class nodes, got %d", classCount)
	}
}
