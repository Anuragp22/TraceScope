package analyzer

import (
	"testing"

	"github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
)

func TestMapDiffToFunctions_BasicOverlap(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "file:1", Type: graph.NodeFile, Name: "main.go", FilePath: "src/main.go"},
			{ID: "func:1", Type: graph.NodeFunction, Name: "hello", FilePath: "src/main.go", StartLine: 5, EndLine: 10},
			{ID: "func:2", Type: graph.NodeFunction, Name: "goodbye", FilePath: "src/main.go", StartLine: 12, EndLine: 20},
		},
	}

	changedFiles := []diff.ChangedFile{
		{Path: "src/main.go", LineRanges: []diff.LineRange{{Start: 7, End: 8}}},
	}

	result := MapDiffToFunctions(changedFiles, gd)
	if len(result) != 1 {
		t.Fatalf("expected 1 changed function, got %d", len(result))
	}
	if result[0].Node.Name != "hello" {
		t.Errorf("expected 'hello', got %q", result[0].Node.Name)
	}
}

func TestMapDiffToFunctions_NoOverlap(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "func:1", Type: graph.NodeFunction, Name: "hello", FilePath: "main.go", StartLine: 5, EndLine: 10},
		},
	}

	changedFiles := []diff.ChangedFile{
		{Path: "main.go", LineRanges: []diff.LineRange{{Start: 1, End: 3}}},
	}

	result := MapDiffToFunctions(changedFiles, gd)
	if len(result) != 0 {
		t.Errorf("expected 0 changed functions, got %d", len(result))
	}
}

func TestMapDiffToFunctions_NewFile(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "func:1", Type: graph.NodeFunction, Name: "a", FilePath: "new.go", StartLine: 1, EndLine: 5},
			{ID: "func:2", Type: graph.NodeFunction, Name: "b", FilePath: "new.go", StartLine: 7, EndLine: 10},
		},
	}

	changedFiles := []diff.ChangedFile{
		{Path: "new.go", IsNew: true},
	}

	result := MapDiffToFunctions(changedFiles, gd)
	if len(result) != 2 {
		t.Errorf("expected 2 changed functions for new file, got %d", len(result))
	}
}

func TestMapDiffToFunctions_PathSegmentMatching(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "func:1", Type: graph.NodeFunction, Name: "helper", FilePath: "project/src/utils/helper.go", StartLine: 1, EndLine: 10},
		},
	}

	// Diff path is relative, graph path is absolute-ish
	changedFiles := []diff.ChangedFile{
		{Path: "src/utils/helper.go", LineRanges: []diff.LineRange{{Start: 3, End: 5}}},
	}

	result := MapDiffToFunctions(changedFiles, gd)
	if len(result) != 1 {
		t.Errorf("expected 1 match via path-segment suffix, got %d", len(result))
	}
}

func TestMapDiffToFunctions_Deduplicate(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "func:1", Type: graph.NodeFunction, Name: "process", FilePath: "app.go", StartLine: 5, EndLine: 20},
		},
	}

	// Two line ranges in the same file overlapping the same function
	changedFiles := []diff.ChangedFile{
		{Path: "app.go", LineRanges: []diff.LineRange{
			{Start: 7, End: 8},
			{Start: 15, End: 16},
		}},
	}

	result := MapDiffToFunctions(changedFiles, gd)
	if len(result) != 1 {
		t.Errorf("expected 1 (deduplicated) changed function, got %d", len(result))
	}
}

func TestMapDiffToFileNodes(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "file:1", Type: graph.NodeFile, FilePath: "src/main.go"},
			{ID: "file:2", Type: graph.NodeFile, FilePath: "src/utils.go"},
		},
	}

	changedFiles := []diff.ChangedFile{
		{Path: "src/main.go"},
	}

	result := MapDiffToFileNodes(changedFiles, gd)
	if len(result) != 1 || result[0] != "file:1" {
		t.Errorf("expected [file:1], got %v", result)
	}
}
