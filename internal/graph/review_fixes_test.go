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

	if alpha.EndLine != 4 {
		t.Errorf("alpha node EndLine = %d, want 4 — registerSymbolDefinitions must "+
			"adopt the enclosing range as the definition's end line", alpha.EndLine)
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

// Blast-radius flood: the SCIP indexer emits CALLS edges from a function to
// the *type* it references (e.g. a `*Context` parameter). The reverse BFS
// must not follow those edges — otherwise reaching a class node via its
// changed member turns that class into a hub that floods the blast radius
// with every type user. A real call always targets a function.
func TestComputeBlastRadius_DoesNotFloodThroughClassNode(t *testing.T) {
	gd := &GraphData{
		Nodes: []Node{
			{ID: "widget", Type: NodeClass, Name: "Widget"},
			{ID: "render", Type: NodeFunction, Name: "Render"},
			{ID: "realCaller", Type: NodeFunction, Name: "RealCaller"},
			{ID: "typeUserA", Type: NodeFunction, Name: "TypeUserA"},
			{ID: "typeUserB", Type: NodeFunction, Name: "TypeUserB"},
		},
		Edges: []Edge{
			// Render is a method of Widget — Widget is its container.
			{Source: "widget", Target: "render", Type: EdgeContains},
			// A genuine caller of the changed function.
			{Source: "realCaller", Target: "render", Type: EdgeCalls},
			// These only reference the Widget *type* — the indexer models that
			// as a CALLS edge into the class node. They do not call Render.
			{Source: "typeUserA", Target: "widget", Type: EdgeCalls},
			{Source: "typeUserB", Target: "widget", Type: EdgeCalls},
		},
	}

	br := ComputeBlastRadius(gd, []string{"render"}, 5)

	if _, ok := br.AffectedNodes["realCaller"]; !ok {
		t.Error("RealCaller genuinely calls Render — it must be in the blast radius")
	}
	for _, id := range []string{"typeUserA", "typeUserB"} {
		if _, ok := br.AffectedNodes[id]; ok {
			t.Errorf("%s only references the Widget type — it must NOT be in the blast "+
				"radius (the class node must not act as a traversal hub)", id)
		}
	}
}

// SCIP function bounds: scip-go emits identifier-only definition ranges, so a
// Go function node would have EndLine == StartLine. That breaks two things — a
// body-only diff change would not overlap the node, and a file-scope reference
// past the function would be misattributed to it. The builder must widen Go
// function nodes to their real body end via the native Go parser.
func TestBuildFromSCIP_RefinesGoFunctionBounds(t *testing.T) {
	dir := t.TempDir()

	// Helper spans lines 3-5, Target spans lines 7-9. The reference on line 11
	// (`var Sink = Helper()`) is at file scope — past the end of Target.
	src := "package sample\n" + // 1
		"\n" + // 2
		"func Helper() int {\n" + // 3
		"\treturn 42\n" + // 4
		"}\n" + // 5
		"\n" + // 6
		"func Target() int {\n" + // 7
		"\treturn Helper()\n" + // 8
		"}\n" + // 9
		"\n" + // 10
		"var Sink = Helper()\n" // 11
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	helperSym := "scip-go gomod example.com/sample pkg Helper()."
	targetSym := "scip-go gomod example.com/sample pkg Target()."
	def := func(line int32, sym string) *scip.Occurrence {
		return &scip.Occurrence{Range: []int32{line, 5, 11}, Symbol: sym, SymbolRoles: int32(scip.SymbolRole_Definition)}
	}
	ref := func(line, col int32, sym string) *scip.Occurrence {
		return &scip.Occurrence{Range: []int32{line, col, col + 6}, Symbol: sym}
	}
	index := &scip.Index{
		Documents: []*scip.Document{
			{
				RelativePath: "sample.go",
				Language:     "go",
				Occurrences: []*scip.Occurrence{
					def(2, helperSym),      // Helper definition, line 3
					def(6, targetSym),      // Target definition, line 7
					ref(7, 8, helperSym),   // Helper() call inside Target's body, line 8
					ref(10, 11, helperSym), // Helper() at file scope, line 11
				},
				Symbols: []*scip.SymbolInformation{
					{Symbol: helperSym, Kind: scip.SymbolInformation_Function, DisplayName: "Helper"},
					{Symbol: targetSym, Kind: scip.SymbolInformation_Function, DisplayName: "Target"},
				},
			},
		},
	}

	// The index must sit next to the source so the builder can load it.
	indexPath := filepath.Join(dir, "index.scip")
	raw, err := proto.Marshal(index)
	if err != nil {
		t.Fatalf("marshal scip index: %v", err)
	}
	if err := os.WriteFile(indexPath, raw, 0o600); err != nil {
		t.Fatalf("write scip index: %v", err)
	}

	gd, err := BuildFromSCIP(indexPath)
	if err != nil {
		t.Fatalf("BuildFromSCIP failed: %v", err)
	}

	var target, helper, file *Node
	for i := range gd.Nodes {
		switch n := &gd.Nodes[i]; {
		case n.Name == "Target" && n.Type == NodeFunction:
			target = n
		case n.Name == "Helper" && n.Type == NodeFunction:
			helper = n
		case n.Type == NodeFile && n.FilePath == "sample.go":
			file = n
		}
	}
	if target == nil || helper == nil || file == nil {
		t.Fatalf("expected Target/Helper/file nodes, got %+v", gd.Nodes)
	}

	// (1) End line widened to the real closing brace.
	if target.EndLine != 9 {
		t.Errorf("Target EndLine = %d, want 9 (closing brace) — Go function bounds "+
			"were not refined from the native parser", target.EndLine)
	}

	// (2) The in-body call is attributed to Target.
	if !hasEdge(gd, target.ID, helper.ID, EdgeCalls) {
		t.Error("the Helper() call inside Target's body should produce Target --CALLS--> Helper")
	}

	// (3) The file-scope reference past Target's body must be attributed to the
	// file, not misattributed to the nearest preceding function.
	if !hasEdge(gd, file.ID, helper.ID, EdgeCalls) {
		t.Error("the file-scope Helper() reference should be attributed to the file node, " +
			"not to Target (refined bounds let enclosingSCIPScopeID reject it)")
	}
}

// TestBuilder_DeduplicatesCallEdges pins one CALLS edge per (caller, callee)
// pair rather than one per call site. Caller counts are derived by counting
// edges, so duplicates report a single caller as many, which can push a
// function over the HIGH threshold by itself. The SCIP backend already dedups
// (scipGraphBuilder.addEdge); without this the two backends disagree about risk
// for identical code.
func TestBuilder_DeduplicatesCallEdges(t *testing.T) {
	fr := &parser.FileResult{
		FilePath: "/repo/a.go",
		Language: parser.LangGo,
		Package:  "a",
		Functions: []parser.FunctionDef{
			{Name: "caller", StartLine: 1, EndLine: 10},
			{Name: "target", StartLine: 12, EndLine: 14},
		},
		Calls: []parser.FunctionCall{
			{Name: "target", Line: 2},
			{Name: "target", Line: 3},
			{Name: "target", Line: 4},
		},
	}
	gd := NewBuilder().Build([]*parser.FileResult{fr})

	var targetID string
	for _, n := range gd.Nodes {
		if n.Name == "target" {
			targetID = n.ID
		}
	}
	edges := 0
	for _, e := range gd.Edges {
		if e.Type == EdgeCalls && e.Target == targetID {
			edges++
		}
	}
	if edges != 1 {
		t.Errorf("expected 1 CALLS edge from a single caller, got %d — caller counts would be inflated", edges)
	}

	// The per-call-site diagnostic must still count every reference: three call
	// sites resolved, even though they collapse to one edge.
	if gd.ResolutionStats.ExactCallEdges != 3 {
		t.Errorf("expected 3 resolved call sites in ResolutionStats, got %d", gd.ResolutionStats.ExactCallEdges)
	}
}

// TestBuilder_CallEdgeConfidenceUpgrades asserts that when the same pair is
// reached by both a heuristic and an exact resolution, the surviving edge is
// exact — an exact resolution anywhere is stronger evidence the edge is real.
func TestBuilder_CallEdgeConfidenceUpgrades(t *testing.T) {
	fr := &parser.FileResult{
		FilePath: "/repo/a.go",
		Language: parser.LangGo,
		Package:  "a",
		Functions: []parser.FunctionDef{
			{Name: "caller", StartLine: 1, EndLine: 10},
			{Name: "target", StartLine: 12, EndLine: 14},
		},
		Calls: []parser.FunctionCall{
			{Name: "target", Line: 2, Receiver: "obj"}, // heuristic path
			{Name: "target", Line: 3},                  // exact, same file
		},
	}
	gd := NewBuilder().Build([]*parser.FileResult{fr})
	for _, e := range gd.Edges {
		if e.Type == EdgeCalls && e.Confidence != EdgeConfidenceExact {
			t.Errorf("expected the surviving edge to be upgraded to EXACT, got %q", e.Confidence)
		}
	}
}

// TestBuilder_ReceiverCallIsNotExactViaBareName pins that a call with a
// receiver, once every receiver-aware strategy has failed, is not reported as
// an exact resolution. It may still match on the bare name — a same-file method
// call whose receiver type could not be inferred, say — but `x.Foo()` matching
// some definition of `Foo` chosen without knowing what `x` is has not been
// resolved with certainty, and the review score should discount it.
func TestBuilder_ReceiverCallIsNotExactViaBareName(t *testing.T) {
	fr := &parser.FileResult{
		FilePath: "/repo/a.go",
		Language: parser.LangGo,
		Package:  "a",
		Functions: []parser.FunctionDef{
			{Name: "caller", StartLine: 1, EndLine: 10},
			{Name: "Error", StartLine: 12, EndLine: 14, Receiver: "MyErr"},
		},
		Calls: []parser.FunctionCall{
			{Name: "Error", Line: 2, Receiver: "err"}, // err.Error() — unknown receiver
		},
	}
	gd := NewBuilder().Build([]*parser.FileResult{fr})
	for _, e := range gd.Edges {
		if e.Type == EdgeCalls && e.Confidence == EdgeConfidenceExact {
			t.Errorf("err.Error() resolved to a same-package Error with EXACT confidence; "+
				"receiver was never identified, so this should be heuristic")
		}
	}

	// A genuinely bare call in the same file stays exact — the downgrade must
	// key on the presence of a receiver, not on bare-name matching generally.
	fr2 := &parser.FileResult{
		FilePath:  "/repo/b.go",
		Language:  parser.LangGo,
		Package:   "b",
		Functions: []parser.FunctionDef{{Name: "caller", StartLine: 1, EndLine: 5}, {Name: "helper", StartLine: 7, EndLine: 9}},
		Calls:     []parser.FunctionCall{{Name: "helper", Line: 2}},
	}
	gd2 := NewBuilder().Build([]*parser.FileResult{fr2})
	found := false
	for _, e := range gd2.Edges {
		if e.Type == EdgeCalls {
			found = true
			if e.Confidence != EdgeConfidenceExact {
				t.Errorf("a bare same-file call should stay EXACT, got %q", e.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected a CALLS edge for the bare same-file call")
	}
}
