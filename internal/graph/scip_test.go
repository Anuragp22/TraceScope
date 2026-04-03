package graph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anurag/tracescope/internal/parser"
	scip "github.com/scip-code/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
)

func TestBuildFromSCIP_MapsDefinitionsAndReferences(t *testing.T) {
	indexPath := filepath.Join(t.TempDir(), "index.scip")
	index := &scip.Index{
		Documents: []*scip.Document{
			{
				RelativePath: "service.go",
				Language:     "go",
				Occurrences: []*scip.Occurrence{
					{
						Range:       []int32{0, 5, 8},
						Symbol:      "scip-go gomod example.com/acme/pkg Service#",
						SymbolRoles: int32(scip.SymbolRole_Definition),
					},
					{
						Range:       []int32{1, 5, 8},
						Symbol:      "scip-go gomod example.com/acme/pkg Service#Run().",
						SymbolRoles: int32(scip.SymbolRole_Definition),
					},
				},
				Symbols: []*scip.SymbolInformation{
					{
						Symbol:      "scip-go gomod example.com/acme/pkg Service#",
						Kind:        scip.SymbolInformation_Class,
						DisplayName: "Service",
					},
					{
						Symbol:          "scip-go gomod example.com/acme/pkg Service#Run().",
						Kind:            scip.SymbolInformation_Method,
						DisplayName:     "Run",
						EnclosingSymbol: "scip-go gomod example.com/acme/pkg Service#",
					},
				},
			},
			{
				RelativePath: "main.go",
				Language:     "go",
				Occurrences: []*scip.Occurrence{
					{
						Range:       []int32{0, 5, 9},
						Symbol:      "scip-go gomod example.com/acme/cmd main().",
						SymbolRoles: int32(scip.SymbolRole_Definition),
					},
					{
						Range:  []int32{1, 6, 9},
						Symbol: "scip-go gomod example.com/acme/pkg Service#Run().",
					},
				},
				Symbols: []*scip.SymbolInformation{
					{
						Symbol:      "scip-go gomod example.com/acme/cmd main().",
						Kind:        scip.SymbolInformation_Function,
						DisplayName: "main",
					},
				},
			},
		},
	}

	raw, err := proto.Marshal(index)
	if err != nil {
		t.Fatalf("marshal scip fixture: %v", err)
	}
	if err := os.WriteFile(indexPath, raw, 0o600); err != nil {
		t.Fatalf("write scip fixture: %v", err)
	}

	gd, err := BuildFromSCIP(indexPath)
	if err != nil {
		t.Fatalf("BuildFromSCIP failed: %v", err)
	}

	if gd.IndexSource != "scip" {
		t.Fatalf("expected index source scip, got %q", gd.IndexSource)
	}

	var mainNode, runNode, serviceNode *Node
	for i := range gd.Nodes {
		node := &gd.Nodes[i]
		switch {
		case node.Name == "main" && node.FilePath == "main.go":
			mainNode = node
		case node.Name == "Run" && node.FilePath == "service.go":
			runNode = node
		case node.Name == "Service" && node.FilePath == "service.go":
			serviceNode = node
		}
	}
	if mainNode == nil || runNode == nil || serviceNode == nil {
		t.Fatalf("expected main/Run/Service nodes, got %+v", gd.Nodes)
	}

	if !hasEdge(gd, mainNode.ID, runNode.ID, EdgeCalls) {
		t.Fatal("expected main -> Run CALLS edge from SCIP references")
	}
	if !hasEdge(gd, serviceNode.ID, runNode.ID, EdgeContains) {
		t.Fatal("expected Service -> Run CONTAINS edge from SCIP enclosing symbol")
	}
	if !hasEdge(gd, makeFileID("main.go"), mainNode.ID, EdgeContains) {
		t.Fatal("expected main.go -> main CONTAINS edge")
	}
}

func TestBuildFromSCIP_MatchesParserFallbackBlastRadiusShape(t *testing.T) {
	indexPath := filepath.Join(t.TempDir(), "index.scip")
	index := &scip.Index{
		Documents: []*scip.Document{
			{
				RelativePath: "main.go",
				Language:     "go",
				Occurrences: []*scip.Occurrence{
					{
						Range:       []int32{0, 5, 9},
						Symbol:      "scip-go gomod example.com/acme/pkg main().",
						SymbolRoles: int32(scip.SymbolRole_Definition),
					},
					{
						Range:  []int32{1, 8, 11},
						Symbol: "scip-go gomod example.com/acme/pkg Run().",
					},
				},
				Symbols: []*scip.SymbolInformation{
					{
						Symbol:      "scip-go gomod example.com/acme/pkg main().",
						Kind:        scip.SymbolInformation_Function,
						DisplayName: "main",
					},
				},
			},
			{
				RelativePath: "service.go",
				Language:     "go",
				Occurrences: []*scip.Occurrence{{
					Range:       []int32{0, 5, 8},
					Symbol:      "scip-go gomod example.com/acme/pkg Run().",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				}},
				Symbols: []*scip.SymbolInformation{{
					Symbol:      "scip-go gomod example.com/acme/pkg Run().",
					Kind:        scip.SymbolInformation_Function,
					DisplayName: "Run",
				}},
			},
		},
	}

	raw, err := proto.Marshal(index)
	if err != nil {
		t.Fatalf("marshal scip fixture: %v", err)
	}
	if err := os.WriteFile(indexPath, raw, 0o600); err != nil {
		t.Fatalf("write scip fixture: %v", err)
	}

	scipGraph, err := BuildFromSCIP(indexPath)
	if err != nil {
		t.Fatalf("BuildFromSCIP failed: %v", err)
	}

	parserGraph := NewBuilder().Build([]*parser.FileResult{
		{
			FilePath: "main.go",
			Language: parser.LangGo,
			Package:  "pkg",
			Functions: []parser.FunctionDef{
				{Name: "main", StartLine: 1, EndLine: 3},
			},
			Calls: []parser.FunctionCall{
				{Name: "Run", Line: 2},
			},
		},
		{
			FilePath: "service.go",
			Language: parser.LangGo,
			Package:  "pkg",
			Functions: []parser.FunctionDef{
				{Name: "Run", StartLine: 1, EndLine: 3},
			},
		},
	})

	if countNodesByType(scipGraph, NodeFunction) != countNodesByType(parserGraph, NodeFunction) {
		t.Fatalf("function node count mismatch: scip=%d parser=%d",
			countNodesByType(scipGraph, NodeFunction),
			countNodesByType(parserGraph, NodeFunction),
		)
	}
	if countNodesByType(scipGraph, NodeFile) != countNodesByType(parserGraph, NodeFile) {
		t.Fatalf("file node count mismatch: scip=%d parser=%d",
			countNodesByType(scipGraph, NodeFile),
			countNodesByType(parserGraph, NodeFile),
		)
	}

	scipRunID := findNodeIDByNameAndFile(scipGraph, "Run", "service.go")
	parserRunID := findNodeIDByNameAndFile(parserGraph, "Run", "service.go")
	if scipRunID == "" || parserRunID == "" {
		t.Fatal("expected Run nodes in both graphs")
	}

	scipBlast := ComputeBlastRadius(scipGraph, []string{scipRunID}, 3)
	parserBlast := ComputeBlastRadius(parserGraph, []string{parserRunID}, 3)
	if len(scipBlast.AffectedNodes) != len(parserBlast.AffectedNodes) {
		t.Fatalf("blast-radius size mismatch: scip=%d parser=%d", len(scipBlast.AffectedNodes), len(parserBlast.AffectedNodes))
	}
	if findNodeByName(scipBlast.AffectedNodes, "main") == nil || findNodeByName(parserBlast.AffectedNodes, "main") == nil {
		t.Fatal("expected main to be affected in both SCIP and parser graphs")
	}
}

func TestBuildFromSCIPFiles_MergesMultipleIndexes(t *testing.T) {
	dir := t.TempDir()
	goIndexPath := filepath.Join(dir, "go.scip")
	tsIndexPath := filepath.Join(dir, "ts.scip")

	writeSCIPIndexFixture(t, goIndexPath, &scip.Index{
		Documents: []*scip.Document{{
			RelativePath: "service.go",
			Language:     "go",
			Occurrences: []*scip.Occurrence{{
				Range:       []int32{0, 5, 8},
				Symbol:      "scip-go gomod example.com/acme/pkg Run().",
				SymbolRoles: int32(scip.SymbolRole_Definition),
			}},
			Symbols: []*scip.SymbolInformation{{
				Symbol:      "scip-go gomod example.com/acme/pkg Run().",
				Kind:        scip.SymbolInformation_Function,
				DisplayName: "Run",
			}},
		}},
	})
	writeSCIPIndexFixture(t, tsIndexPath, &scip.Index{
		Documents: []*scip.Document{{
			RelativePath: "app.ts",
			Language:     "typescript",
			Occurrences: []*scip.Occurrence{{
				Range:       []int32{0, 16, 21},
				Symbol:      "scip-typescript npm demo 1.0.0 `app.ts`/start().",
				SymbolRoles: int32(scip.SymbolRole_Definition),
			}},
			Symbols: []*scip.SymbolInformation{{
				Symbol:      "scip-typescript npm demo 1.0.0 `app.ts`/start().",
				Kind:        scip.SymbolInformation_Function,
				DisplayName: "start",
			}},
		}},
	})

	gd, err := BuildFromSCIPFiles([]string{goIndexPath, tsIndexPath})
	if err != nil {
		t.Fatalf("BuildFromSCIPFiles failed: %v", err)
	}
	if findNodeIDByNameAndFile(gd, "Run", "service.go") == "" {
		t.Fatal("expected Go symbol from merged SCIP indexes")
	}
	if findNodeIDByNameAndFile(gd, "start", "app.ts") == "" {
		t.Fatal("expected TypeScript symbol from merged SCIP indexes")
	}
}

func TestBuildFromSCIP_MapsInheritanceRelationshipsAndDedupesReferences(t *testing.T) {
	indexPath := filepath.Join(t.TempDir(), "index.scip")
	writeSCIPIndexFixture(t, indexPath, &scip.Index{
		Documents: []*scip.Document{
			{
				RelativePath: "types.ts",
				Language:     "typescript",
				Occurrences: []*scip.Occurrence{
					{
						Range:       []int32{0, 13, 17},
						Symbol:      "scip-typescript npm demo 1.0.0 `types.ts`/Base#",
						SymbolRoles: int32(scip.SymbolRole_Definition),
					},
					{
						Range:       []int32{1, 17, 22},
						Symbol:      "scip-typescript npm demo 1.0.0 `types.ts`/Child#",
						SymbolRoles: int32(scip.SymbolRole_Definition),
					},
					{
						Range:  []int32{1, 25, 29},
						Symbol: "scip-typescript npm demo 1.0.0 `types.ts`/Base#",
					},
				},
				Symbols: []*scip.SymbolInformation{
					{
						Symbol:      "scip-typescript npm demo 1.0.0 `types.ts`/Base#",
						Kind:        scip.SymbolInformation_Class,
						DisplayName: "Base",
					},
					{
						Symbol:      "scip-typescript npm demo 1.0.0 `types.ts`/Child#",
						Kind:        scip.SymbolInformation_Class,
						DisplayName: "Child",
						Relationships: []*scip.Relationship{{
							Symbol:      "scip-typescript npm demo 1.0.0 `types.ts`/Base#",
							IsReference: true,
						}},
					},
				},
			},
			{
				RelativePath: "app.ts",
				Language:     "typescript",
				Occurrences: []*scip.Occurrence{
					{
						Range:       []int32{0, 16, 21},
						Symbol:      "scip-typescript npm demo 1.0.0 `app.ts`/start().",
						SymbolRoles: int32(scip.SymbolRole_Definition),
					},
					{
						Range:  []int32{1, 6, 11},
						Symbol: "scip-typescript npm demo 1.0.0 `types.ts`/Child#",
					},
				},
				Symbols: []*scip.SymbolInformation{{
					Symbol:      "scip-typescript npm demo 1.0.0 `app.ts`/start().",
					Kind:        scip.SymbolInformation_Function,
					DisplayName: "start",
					Relationships: []*scip.Relationship{{
						Symbol:      "scip-typescript npm demo 1.0.0 `types.ts`/Child#",
						IsReference: true,
					}},
				}},
			},
		},
	})

	gd, err := BuildFromSCIP(indexPath)
	if err != nil {
		t.Fatalf("BuildFromSCIP failed: %v", err)
	}

	baseID := findNodeIDByNameAndFile(gd, "Base", "types.ts")
	childID := findNodeIDByNameAndFile(gd, "Child", "types.ts")
	startID := findNodeIDByNameAndFile(gd, "start", "app.ts")
	if baseID == "" || childID == "" || startID == "" {
		t.Fatalf("expected Base/Child/start nodes, got %+v", gd.Nodes)
	}
	if !hasEdge(gd, childID, baseID, EdgeExtends) {
		t.Fatal("expected Child -> Base EXTENDS edge from SCIP relationship")
	}
	if countEdges(gd, startID, childID, EdgeCalls) != 1 {
		t.Fatalf("expected one deduped start -> Child CALLS edge, got %d", countEdges(gd, startID, childID, EdgeCalls))
	}
}

func TestBuildFromSCIP_InfersGoMethodParentAndSkipsVariableSymbols(t *testing.T) {
	indexPath := filepath.Join(t.TempDir(), "index.scip")
	writeSCIPIndexFixture(t, indexPath, &scip.Index{
		Documents: []*scip.Document{{
			RelativePath: "builder.go",
			Language:     "go",
			Occurrences: []*scip.Occurrence{
				{
					Range:       []int32{0, 5, 12},
					Symbol:      "scip-go gomod example.com/acme/pkg Builder#",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
				{
					Range:       []int32{1, 18, 23},
					Symbol:      "scip-go gomod example.com/acme/pkg (*Builder).Build().",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
				{
					Range:       []int32{2, 4, 12},
					Symbol:      "scip-go gomod example.com/acme/pkg replacer.",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
			},
			Symbols: []*scip.SymbolInformation{
				{
					Symbol:      "scip-go gomod example.com/acme/pkg Builder#",
					Kind:        scip.SymbolInformation_Struct,
					DisplayName: "Builder",
				},
				{
					Symbol:      "scip-go gomod example.com/acme/pkg (*Builder).Build().",
					Kind:        scip.SymbolInformation_Method,
					DisplayName: "Build",
				},
				{
					Symbol:      "scip-go gomod example.com/acme/pkg replacer.",
					Kind:        scip.SymbolInformation_Variable,
					DisplayName: "replacer",
				},
			},
		}},
	})

	gd, err := BuildFromSCIP(indexPath)
	if err != nil {
		t.Fatalf("BuildFromSCIP failed: %v", err)
	}

	builderID := findNodeIDByNameAndFile(gd, "Builder", "builder.go")
	buildID := findNodeIDByNameAndFile(gd, "Build", "builder.go")
	if builderID == "" || buildID == "" {
		t.Fatalf("expected Builder and Build nodes, got %+v", gd.Nodes)
	}
	if !hasEdge(gd, builderID, buildID, EdgeContains) {
		t.Fatal("expected Builder -> Build CONTAINS edge")
	}
	if replacerID := findNodeIDByNameAndFile(gd, "replacer", "builder.go"); replacerID != "" {
		t.Fatalf("expected variable symbol to be skipped, got node %s", replacerID)
	}
}

func TestBuildFromSCIP_InfersMethodParentWithoutSymbolInformation(t *testing.T) {
	indexPath := filepath.Join(t.TempDir(), "index.scip")
	writeSCIPIndexFixture(t, indexPath, &scip.Index{
		Documents: []*scip.Document{{
			RelativePath: "builder.go",
			Language:     "go",
			Occurrences: []*scip.Occurrence{
				{
					Range:       []int32{0, 5, 12},
					Symbol:      "scip-go gomod example.com/acme/pkg Builder#",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
				{
					Range:       []int32{1, 18, 23},
					Symbol:      "scip-go gomod example.com/acme/pkg (*Builder).Build().",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
			},
			Symbols: []*scip.SymbolInformation{{
				Symbol:      "scip-go gomod example.com/acme/pkg Builder#",
				Kind:        scip.SymbolInformation_Struct,
				DisplayName: "Builder",
			}},
		}},
	})

	gd, err := BuildFromSCIP(indexPath)
	if err != nil {
		t.Fatalf("BuildFromSCIP failed: %v", err)
	}

	builderID := findNodeIDByNameAndFile(gd, "Builder", "builder.go")
	buildID := findNodeIDByNameAndFile(gd, "Build", "builder.go")
	if builderID == "" || buildID == "" {
		t.Fatalf("expected Builder and Build nodes, got %+v", gd.Nodes)
	}
	if !hasEdge(gd, builderID, buildID, EdgeContains) {
		t.Fatal("expected inferred Builder -> Build CONTAINS edge without method SymbolInformation")
	}
}

func TestBuildFromSCIP_MapsInterfaceMethodsWithoutParameterList(t *testing.T) {
	indexPath := filepath.Join(t.TempDir(), "index.scip")
	writeSCIPIndexFixture(t, indexPath, &scip.Index{
		Documents: []*scip.Document{{
			RelativePath: "parser.go",
			Language:     "go",
			Occurrences: []*scip.Occurrence{
				{
					Range:       []int32{0, 5, 19},
					Symbol:      "scip-go gomod example.com/acme/pkg LanguageParser#",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
				{
					Range:       []int32{1, 1, 6},
					Symbol:      "scip-go gomod example.com/acme/pkg LanguageParser#Parse.",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
				{
					Range:       []int32{2, 1, 9},
					Symbol:      "scip-go gomod example.com/acme/pkg LanguageParser#Language.",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
			},
		}},
	})

	gd, err := BuildFromSCIP(indexPath)
	if err != nil {
		t.Fatalf("BuildFromSCIP failed: %v", err)
	}

	ifaceID := findNodeIDByNameAndFile(gd, "LanguageParser", "parser.go")
	parseID := findNodeIDByNameAndFile(gd, "Parse", "parser.go")
	langID := findNodeIDByNameAndFile(gd, "Language", "parser.go")
	if ifaceID == "" || parseID == "" || langID == "" {
		t.Fatalf("expected LanguageParser/Parse/Language nodes, got %+v", gd.Nodes)
	}
	if !hasEdge(gd, ifaceID, parseID, EdgeContains) {
		t.Fatal("expected LanguageParser -> Parse CONTAINS edge")
	}
	if !hasEdge(gd, ifaceID, langID, EdgeContains) {
		t.Fatal("expected LanguageParser -> Language CONTAINS edge")
	}
}

func TestBuildFromSCIP_SkipsStructFieldsButKeepsInterfaceMethods(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.go"), []byte(`package cfg

type Config struct {
	Enabled bool
}

type Validator interface {
	Validate() error
}
`), 0o600); err != nil {
		t.Fatalf("write config.go: %v", err)
	}

	indexPath := filepath.Join(dir, "index.scip")
	writeSCIPIndexFixture(t, indexPath, &scip.Index{
		Documents: []*scip.Document{{
			RelativePath: "config.go",
			Language:     "go",
			Occurrences: []*scip.Occurrence{
				{
					Range:       []int32{2, 5, 11},
					Symbol:      "scip-go gomod example.com/acme/pkg Config#",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
				{
					Range:       []int32{3, 1, 8},
					Symbol:      "scip-go gomod example.com/acme/pkg Config#Enabled.",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
				{
					Range:       []int32{6, 5, 14},
					Symbol:      "scip-go gomod example.com/acme/pkg Validator#",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
				{
					Range:       []int32{7, 1, 9},
					Symbol:      "scip-go gomod example.com/acme/pkg Validator#Validate.",
					SymbolRoles: int32(scip.SymbolRole_Definition),
				},
			},
		}},
	})

	gd, err := BuildFromSCIP(indexPath)
	if err != nil {
		t.Fatalf("BuildFromSCIP failed: %v", err)
	}

	if fieldID := findNodeIDByNameAndFile(gd, "Enabled", "config.go"); fieldID != "" {
		t.Fatalf("expected struct field Enabled to be skipped, got node %s", fieldID)
	}
	validatorID := findNodeIDByNameAndFile(gd, "Validator", "config.go")
	validateID := findNodeIDByNameAndFile(gd, "Validate", "config.go")
	if validatorID == "" || validateID == "" {
		t.Fatalf("expected Validator and Validate nodes, got %+v", gd.Nodes)
	}
	if !hasEdge(gd, validatorID, validateID, EdgeContains) {
		t.Fatal("expected Validator -> Validate CONTAINS edge")
	}
}

func hasEdge(gd *GraphData, sourceID, targetID string, edgeType EdgeType) bool {
	for _, edge := range gd.Edges {
		if edge.Source == sourceID && edge.Target == targetID && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func countNodesByType(gd *GraphData, nodeType NodeType) int {
	count := 0
	for _, node := range gd.Nodes {
		if node.Type == nodeType {
			count++
		}
	}
	return count
}

func findNodeIDByNameAndFile(gd *GraphData, name, filePath string) string {
	for _, node := range gd.Nodes {
		if node.Name == name && node.FilePath == filePath {
			return node.ID
		}
	}
	return ""
}

func findNodeByName(nodes map[string]*Node, name string) *Node {
	for _, node := range nodes {
		if node != nil && node.Name == name {
			return node
		}
	}
	return nil
}

func countEdges(gd *GraphData, sourceID, targetID string, edgeType EdgeType) int {
	count := 0
	for _, edge := range gd.Edges {
		if edge.Source == sourceID && edge.Target == targetID && edge.Type == edgeType {
			count++
		}
	}
	return count
}

func writeSCIPIndexFixture(t *testing.T, indexPath string, index *scip.Index) {
	t.Helper()

	raw, err := proto.Marshal(index)
	if err != nil {
		t.Fatalf("marshal scip fixture: %v", err)
	}
	if err := os.WriteFile(indexPath, raw, 0o600); err != nil {
		t.Fatalf("write scip fixture: %v", err)
	}
}
