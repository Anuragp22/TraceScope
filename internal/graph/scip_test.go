package graph

import (
	"os"
	"path/filepath"
	"testing"

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

func hasEdge(gd *GraphData, sourceID, targetID string, edgeType EdgeType) bool {
	for _, edge := range gd.Edges {
		if edge.Source == sourceID && edge.Target == targetID && edge.Type == edgeType {
			return true
		}
	}
	return false
}
