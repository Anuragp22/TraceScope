package graph

import (
	"testing"
)

func makePathTestGraph() *GraphData {
	return &GraphData{
		Nodes: []Node{
			{ID: "f1", Type: NodeFunction, Name: "main", Package: "cmd", FilePath: "main.go", Language: "go"},
			{ID: "f2", Type: NodeFunction, Name: "Run", Package: "app", FilePath: "app.go", Language: "go", IsExport: true},
			{ID: "f3", Type: NodeFunction, Name: "Build", Package: "graph", FilePath: "builder.go", Language: "go", IsExport: true},
			{ID: "f4", Type: NodeFunction, Name: "Score", Package: "analyzer", FilePath: "scorer.go", Language: "go", IsExport: true},
			{ID: "f5", Type: NodeFunction, Name: "isolated", Package: "util", FilePath: "util.go", Language: "go"},
		},
		Edges: []Edge{
			{Source: "f1", Target: "f2", Type: EdgeCalls},
			{Source: "f2", Target: "f3", Type: EdgeCalls},
			{Source: "f3", Target: "f4", Type: EdgeCalls},
		},
	}
}

func TestFindShortestPath_Direct(t *testing.T) {
	gd := makePathTestGraph()
	result := FindShortestPath(gd, "f1", "f2", false)

	if !result.Found {
		t.Fatal("expected path to be found")
	}
	if result.Length != 1 {
		t.Errorf("expected length 1, got %d", result.Length)
	}
	if len(result.Path) != 2 {
		t.Errorf("expected 2 steps, got %d", len(result.Path))
	}
	if result.Path[0].Node.Name != "main" || result.Path[1].Node.Name != "Run" {
		t.Error("unexpected path nodes")
	}
}

func TestFindShortestPath_Transitive(t *testing.T) {
	gd := makePathTestGraph()
	result := FindShortestPath(gd, "f1", "f4", false)

	if !result.Found {
		t.Fatal("expected path to be found")
	}
	if result.Length != 3 {
		t.Errorf("expected length 3, got %d", result.Length)
	}
	// Path should be: main → Run → Build → Score
	names := make([]string, len(result.Path))
	for i, s := range result.Path {
		names[i] = s.Node.Name
	}
	expected := []string{"main", "Run", "Build", "Score"}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("step %d: expected %q, got %q", i, name, names[i])
		}
	}
}

func TestFindShortestPath_NoPath(t *testing.T) {
	gd := makePathTestGraph()
	result := FindShortestPath(gd, "f1", "f5", false)

	if result.Found {
		t.Error("expected no path to isolated node")
	}
	if result.Message == "" {
		t.Error("expected a message explaining no path")
	}
}

func TestFindShortestPath_SelfReference(t *testing.T) {
	gd := makePathTestGraph()
	result := FindShortestPath(gd, "f1", "f1", false)

	if !result.Found {
		t.Fatal("expected self-path to be found")
	}
	if result.Length != 0 {
		t.Errorf("expected length 0 for self-reference, got %d", result.Length)
	}
}

func TestFindShortestPath_Reverse(t *testing.T) {
	gd := makePathTestGraph()
	// In reverse mode: from Score back to main
	result := FindShortestPath(gd, "f4", "f1", true)

	if !result.Found {
		t.Fatal("expected reverse path to be found")
	}
	if result.Length != 3 {
		t.Errorf("expected length 3, got %d", result.Length)
	}
	// Path should be: Score → Build → Run → main (reverse of call chain)
	if result.Path[0].Node.Name != "Score" || result.Path[3].Node.Name != "main" {
		t.Error("unexpected reverse path endpoints")
	}
}

func TestFindShortestPath_Cycle(t *testing.T) {
	gd := &GraphData{
		Nodes: []Node{
			{ID: "a", Type: NodeFunction, Name: "A", FilePath: "a.go"},
			{ID: "b", Type: NodeFunction, Name: "B", FilePath: "b.go"},
			{ID: "c", Type: NodeFunction, Name: "C", FilePath: "c.go"},
		},
		Edges: []Edge{
			{Source: "a", Target: "b", Type: EdgeCalls},
			{Source: "b", Target: "c", Type: EdgeCalls},
			{Source: "c", Target: "a", Type: EdgeCalls}, // cycle
		},
	}

	result := FindShortestPath(gd, "a", "c", false)
	if !result.Found {
		t.Fatal("expected path even with cycle")
	}
	if result.Length != 2 {
		t.Errorf("expected length 2, got %d", result.Length)
	}
}

func TestFindNodesByName_Exact(t *testing.T) {
	gd := makePathTestGraph()
	matches := FindNodesByName(gd, "Build")

	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}
	if matches[0].Node.Name != "Build" {
		t.Errorf("expected 'Build', got %q", matches[0].Node.Name)
	}
	if matches[0].MatchType != "exact" {
		t.Errorf("expected 'exact' match type, got %q", matches[0].MatchType)
	}
}

func TestFindNodesByName_Qualified(t *testing.T) {
	gd := makePathTestGraph()
	matches := FindNodesByName(gd, "graph.Build")

	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}
	if matches[0].Node.Name != "Build" || matches[0].Node.Package != "graph" {
		t.Errorf("expected graph.Build, got %s.%s", matches[0].Node.Package, matches[0].Node.Name)
	}
}

func TestFindNodesByName_CaseInsensitive(t *testing.T) {
	gd := makePathTestGraph()
	matches := FindNodesByName(gd, "build")

	if len(matches) == 0 {
		t.Fatal("expected case-insensitive match")
	}
	found := false
	for _, m := range matches {
		if m.Node.Name == "Build" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Build' to match case-insensitively")
	}
}

func TestFindNodesByName_NoMatch(t *testing.T) {
	gd := makePathTestGraph()
	matches := FindNodesByName(gd, "NonExistent")

	if len(matches) != 0 {
		t.Errorf("expected no matches, got %d", len(matches))
	}
}

func TestFindNodesByName_Empty(t *testing.T) {
	gd := makePathTestGraph()
	matches := FindNodesByName(gd, "")

	if len(matches) != 0 {
		t.Errorf("expected no matches for empty query, got %d", len(matches))
	}
}
