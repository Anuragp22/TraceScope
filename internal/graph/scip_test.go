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
