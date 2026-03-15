package graph

import (
	"testing"

	"github.com/anurag/tracescope/internal/parser"
)

func TestBuilder_ExtendsEdges(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "models.go",
			Language: parser.LangGo,
			Package:  "models",
			Classes: []parser.ClassDef{
				{Name: "Base", StartLine: 1, EndLine: 5, Kind: "struct"},
				{Name: "User", StartLine: 7, EndLine: 12, Kind: "struct", Bases: []string{"Base"}},
			},
		},
	}

	b := NewBuilder()
	gd := b.Build(results)

	// Find EXTENDS edge: User → Base
	found := false
	for _, e := range gd.Edges {
		if e.Type == EdgeExtends {
			found = true
			// Verify source is User, target is Base
			sourceNode := findNode(gd, e.Source)
			targetNode := findNode(gd, e.Target)
			if sourceNode == nil || targetNode == nil {
				t.Error("EXTENDS edge references missing nodes")
				continue
			}
			if sourceNode.Name != "User" || targetNode.Name != "Base" {
				t.Errorf("expected User extends Base, got %s extends %s", sourceNode.Name, targetNode.Name)
			}
		}
	}
	if !found {
		t.Error("expected EXTENDS edge from User to Base")
	}
}

func TestBuilder_ClassMethodContains(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "models.go",
			Language: parser.LangGo,
			Package:  "models",
			Classes: []parser.ClassDef{
				{Name: "User", StartLine: 1, EndLine: 5, Kind: "struct"},
			},
			Functions: []parser.FunctionDef{
				{Name: "GetName", StartLine: 7, EndLine: 10, Receiver: "User", IsExport: true},
			},
		},
	}

	b := NewBuilder()
	gd := b.Build(results)

	// Should have class→method CONTAINS edge
	classContainsMethod := false
	for _, e := range gd.Edges {
		if e.Type == EdgeContains {
			src := findNode(gd, e.Source)
			tgt := findNode(gd, e.Target)
			if src != nil && tgt != nil && src.Type == NodeClass && tgt.Type == NodeFunction {
				if src.Name == "User" && tgt.Name == "GetName" {
					classContainsMethod = true
				}
			}
		}
	}
	if !classContainsMethod {
		t.Error("expected CONTAINS edge from class User to method GetName")
	}
}

func TestBuilder_FileMetadata(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath:    "main.go",
			Language:    parser.LangGo,
			Package:     "main",
			ContentHash: "abc123",
			Functions: []parser.FunctionDef{
				{Name: "main", StartLine: 1, EndLine: 5},
			},
		},
	}

	b := NewBuilder()
	gd := b.Build(results)

	if gd.FileMetadata == nil {
		t.Fatal("expected FileMetadata to be populated")
	}
	meta, ok := gd.FileMetadata["main.go"]
	if !ok {
		t.Fatal("expected metadata for main.go")
	}
	if meta.Hash != "abc123" {
		t.Errorf("expected hash 'abc123', got %q", meta.Hash)
	}
	if meta.NodeCount != 1 {
		t.Errorf("expected node count 1, got %d", meta.NodeCount)
	}
	if meta.Language != "go" {
		t.Errorf("expected language 'go', got %q", meta.Language)
	}
}

func findNode(gd *GraphData, id string) *Node {
	for i := range gd.Nodes {
		if gd.Nodes[i].ID == id {
			return &gd.Nodes[i]
		}
	}
	return nil
}
