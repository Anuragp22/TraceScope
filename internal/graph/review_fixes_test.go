package graph

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/anurag/tracescope/internal/parser"
	scip "github.com/scip-code/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
)

// writeSCIPFixture marshals an index to a temp .scip file and returns its path.
func writeSCIPFixture(t *testing.T, index *scip.Index) string {
	t.Helper()
	indexPath := filepath.Join(t.TempDir(), "index.scip")
	raw, err := proto.Marshal(index)
	if err != nil {
		t.Fatalf("marshal scip fixture: %v", err)
	}
	if err := os.WriteFile(indexPath, raw, 0o600); err != nil {
		t.Fatalf("write scip fixture: %v", err)
	}
	return indexPath
}

// Bug #1: a reference that sits outside a function's body must not be
// attributed to that function. enclosingSCIPScopeID must honor the
// enclosing_range end line, not just the start line.
func TestBuildFromSCIP_ScopeAttributionRespectsEnclosingRange(t *testing.T) {
	helperSym := "scip-typescript npm pkg 1.0.0 src/`util.ts`/helper()."
	alphaSym := "scip-typescript npm pkg 1.0.0 src/`mod.ts`/alpha()."

	index := &scip.Index{
		Documents: []*scip.Document{
			{
				RelativePath: "util.ts",
				Language:     "typescript",
				Occurrences: []*scip.Occurrence{
					{
						Range:       []int32{0, 16, 22},
						Symbol:      helperSym,
						SymbolRoles: int32(scip.SymbolRole_Definition),
					},
				},
				Symbols: []*scip.SymbolInformation{
					{Symbol: helperSym, Kind: scip.SymbolInformation_Function, DisplayName: "helper"},
				},
			},
			{
				RelativePath: "mod.ts",
				Language:     "typescript",
				Occurrences: []*scip.Occurrence{
					{
						// alpha's body spans lines 2-4 (0-based 1..3).
						Range:          []int32{1, 9, 14},
						EnclosingRange: []int32{1, 0, 3, 1},
						Symbol:         alphaSym,
						SymbolRoles:    int32(scip.SymbolRole_Definition),
					},
					{
						// Reference to helper on line 7 (0-based 6) — file scope,
						// well past the end of alpha's body.
						Range:  []int32{6, 4, 10},
						Symbol: helperSym,
					},
				},
				Symbols: []*scip.SymbolInformation{
					{Symbol: alphaSym, Kind: scip.SymbolInformation_Function, DisplayName: "alpha"},
				},
			},
		},
	}

	gd, err := BuildFromSCIP(writeSCIPFixture(t, index))
	if err != nil {
		t.Fatalf("BuildFromSCIP failed: %v", err)
	}

	var alpha, helper, modFile *Node
	for i := range gd.Nodes {
		n := &gd.Nodes[i]
		switch {
		case n.Name == "alpha":
			alpha = n
		case n.Name == "helper":
			helper = n
		case n.Type == NodeFile && n.FilePath == "mod.ts":
			modFile = n
		}
	}
	if alpha == nil || helper == nil || modFile == nil {
		t.Fatalf("expected alpha/helper/mod.ts nodes, got %+v", gd.Nodes)
	}

	if hasEdge(gd, alpha.ID, helper.ID, EdgeCalls) {
		t.Error("file-scope reference was wrongly attributed to alpha (endLine ignored)")
	}
	if !hasEdge(gd, modFile.ID, helper.ID, EdgeCalls) {
		t.Error("file-scope reference should be attributed to the mod.ts file node")
	}
}

// Bug #7a: class→method CONTAINS edges are emitted while iterating a map,
// so edge order must still be deterministic across builds.
func TestBuildFromSCIP_DeterministicEdgeOrder(t *testing.T) {
	sym := func(s string) string { return "scip-go gomod example.com/ex pkg " + s }
	def := func(line int32, s string) *scip.Occurrence {
		return &scip.Occurrence{Range: []int32{line, 0, 8}, Symbol: s, SymbolRoles: int32(scip.SymbolRole_Definition)}
	}
	index := &scip.Index{
		Documents: []*scip.Document{
			{
				RelativePath: "svc.go",
				Language:     "go",
				Occurrences: []*scip.Occurrence{
					def(0, sym("Alpha#")),
					def(1, sym("Alpha#one().")),
					def(2, sym("Alpha#two().")),
					def(3, sym("Beta#")),
					def(4, sym("Beta#one().")),
					def(5, sym("Beta#two().")),
				},
				Symbols: []*scip.SymbolInformation{
					{Symbol: sym("Alpha#"), Kind: scip.SymbolInformation_Class, DisplayName: "Alpha"},
					{Symbol: sym("Alpha#one()."), Kind: scip.SymbolInformation_Method, DisplayName: "one", EnclosingSymbol: sym("Alpha#")},
					{Symbol: sym("Alpha#two()."), Kind: scip.SymbolInformation_Method, DisplayName: "two", EnclosingSymbol: sym("Alpha#")},
					{Symbol: sym("Beta#"), Kind: scip.SymbolInformation_Class, DisplayName: "Beta"},
					{Symbol: sym("Beta#one()."), Kind: scip.SymbolInformation_Method, DisplayName: "one", EnclosingSymbol: sym("Beta#")},
					{Symbol: sym("Beta#two()."), Kind: scip.SymbolInformation_Method, DisplayName: "two", EnclosingSymbol: sym("Beta#")},
				},
			},
		},
	}
	indexPath := writeSCIPFixture(t, index)

	var first []Edge
	for i := 0; i < 40; i++ {
		gd, err := BuildFromSCIP(indexPath)
		if err != nil {
			t.Fatalf("BuildFromSCIP failed: %v", err)
		}
		if first == nil {
			first = gd.Edges
			continue
		}
		if !reflect.DeepEqual(first, gd.Edges) {
			t.Fatalf("edge order is not deterministic across builds (run %d differs from run 0)", i)
		}
	}
}

// Bug #5: a concrete type that lists an interface as a base implements it —
// the edge must be IMPLEMENTS, not EXTENDS.
func TestBuilder_ImplementsEdgeForInterfaceBase(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "shapes.go",
			Language: parser.LangGo,
			Package:  "shapes",
			Classes: []parser.ClassDef{
				{Name: "Shape", StartLine: 1, EndLine: 3, Kind: "interface"},
				{Name: "Circle", StartLine: 5, EndLine: 9, Kind: "struct", Bases: []string{"Shape"}},
			},
		},
	}

	gd := NewBuilder().Build(results)

	var circle, shape *Node
	for i := range gd.Nodes {
		switch gd.Nodes[i].Name {
		case "Circle":
			circle = &gd.Nodes[i]
		case "Shape":
			shape = &gd.Nodes[i]
		}
	}
	if circle == nil || shape == nil {
		t.Fatalf("expected Circle and Shape nodes, got %+v", gd.Nodes)
	}

	if !hasEdge(gd, circle.ID, shape.ID, EdgeImplements) {
		t.Error("expected Circle --IMPLEMENTS--> Shape (interface base)")
	}
	if hasEdge(gd, circle.ID, shape.ID, EdgeExtends) {
		t.Error("Circle implementing an interface must not produce an EXTENDS edge")
	}
}

// Bug #10: two imports that resolve to the same target file must produce
// exactly one IMPORTS edge.
func TestBuilder_DeduplicatesImportEdges(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "pkg/util/util.go",
			Language: parser.LangGo,
			Package:  "util",
			Functions: []parser.FunctionDef{
				{Name: "Helper", StartLine: 1, EndLine: 3, IsExport: true},
			},
		},
		{
			FilePath: "app/main.go",
			Language: parser.LangGo,
			Package:  "main",
			Imports: []parser.Import{
				{Path: "example.com/pkg/util", Line: 1},
				{Path: "example.com/pkg/util", Line: 2},
			},
		},
	}

	gd := NewBuilder().Build(results)

	imports := 0
	for _, e := range gd.Edges {
		if e.Type == EdgeImports {
			imports++
		}
	}
	if imports != 1 {
		t.Errorf("expected exactly 1 IMPORTS edge after dedup, got %d", imports)
	}
}
