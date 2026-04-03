package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anurag/tracescope/internal/graph"
	scip "github.com/scip-code/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
)

func TestRunIndex_PrefersSCIPWhenPresent(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.scip")

	index := &scip.Index{
		Documents: []*scip.Document{{
			RelativePath: "main.go",
			Language:     "go",
			Occurrences: []*scip.Occurrence{{
				Range:       []int32{0, 5, 9},
				Symbol:      "scip-go gomod example.com/acme/cmd main().",
				SymbolRoles: int32(scip.SymbolRole_Definition),
			}},
			Symbols: []*scip.SymbolInformation{{
				Symbol:      "scip-go gomod example.com/acme/cmd main().",
				Kind:        scip.SymbolInformation_Function,
				DisplayName: "main",
			}},
		}},
	}

	raw, err := proto.Marshal(index)
	if err != nil {
		t.Fatalf("marshal scip fixture: %v", err)
	}
	if err := os.WriteFile(indexPath, raw, 0o600); err != nil {
		t.Fatalf("write index.scip: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	if err := runIndex(indexCmd, []string{dir}); err != nil {
		t.Fatalf("runIndex failed: %v", err)
	}

	gd, err := graph.NewStore().Load(filepath.Join(dir, ".tracescope", "graph.json"))
	if err != nil {
		t.Fatalf("load graph.json: %v", err)
	}
	if gd.IndexSource != "scip" {
		t.Fatalf("expected scip index source, got %q", gd.IndexSource)
	}
}

func TestRunIndex_FallsBackToParserWithoutSCIP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`
package main

func main() {}
`), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	if err := runIndex(indexCmd, []string{dir}); err != nil {
		t.Fatalf("runIndex failed: %v", err)
	}

	gd, err := graph.NewStore().Load(filepath.Join(dir, ".tracescope", "graph.json"))
	if err != nil {
		t.Fatalf("load graph.json: %v", err)
	}
	if gd.IndexSource != "parser" {
		t.Fatalf("expected parser index source, got %q", gd.IndexSource)
	}
	if len(gd.Nodes) == 0 {
		t.Fatal("expected parser fallback graph nodes")
	}
}
