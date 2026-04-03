package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	scip "github.com/scip-code/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
)

// BuildFromSCIP loads an index.scip file and converts it into GraphData.
func BuildFromSCIP(indexPath string) (*GraphData, error) {
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("reading SCIP index: %w", err)
	}

	var index scip.Index
	if err := proto.Unmarshal(raw, &index); err != nil {
		return nil, fmt.Errorf("decoding SCIP index: %w", err)
	}

	builder := scipGraphBuilder{
		graphData:      &GraphData{IndexSource: "scip", FileMetadata: map[string]*FileMetadata{}},
		nodeMap:        map[string]*Node{},
		fileNodeByPath: map[string]string{},
		symbolNodeByID: map[string]string{},
		symbolInfoByID: map[string]*scip.SymbolInformation{},
	}
	builder.collectSymbolInfo(index.GetDocuments(), index.GetExternalSymbols())
	builder.registerDocuments(index.GetDocuments())
	builder.registerSymbolDefinitions(index.GetDocuments())
	builder.registerSymbolRelationships()
	builder.registerReferenceEdges(index.GetDocuments())
	builder.flushNodes()
	return builder.graphData, nil
}

type scipGraphBuilder struct {
	graphData      *GraphData
	nodeMap        map[string]*Node
	fileNodeByPath map[string]string
	symbolNodeByID map[string]string
	symbolInfoByID map[string]*scip.SymbolInformation
}

type scipDefinitionScope struct {
	startLine int
	endLine   int
	nodeID    string
}

func (b *scipGraphBuilder) collectSymbolInfo(documents []*scip.Document, externalSymbols []*scip.SymbolInformation) {
	for _, doc := range documents {
		for _, symbolInfo := range doc.GetSymbols() {
			if symbol := symbolInfo.GetSymbol(); symbol != "" {
				b.symbolInfoByID[symbol] = symbolInfo
			}
		}
	}
	for _, symbolInfo := range externalSymbols {
		if symbol := symbolInfo.GetSymbol(); symbol != "" {
			b.symbolInfoByID[symbol] = symbolInfo
		}
	}
}

func (b *scipGraphBuilder) registerDocuments(documents []*scip.Document) {
	for _, doc := range documents {
		path := canonicalPath(doc.GetRelativePath())
		if path == "" {
			continue
		}
		fileID := makeFileID(path)
		b.fileNodeByPath[path] = fileID
		b.nodeMap[fileID] = &Node{
			ID:       fileID,
			Type:     NodeFile,
			Name:     filepath.Base(path),
			FilePath: path,
			Language: doc.GetLanguage(),
			IsTest:   isSCIPTestFile(path),
		}
		b.graphData.FileMetadata[path] = &FileMetadata{
			Language:  doc.GetLanguage(),
			ParsedAt:  time.Now().Unix(),
			NodeCount: countSCIPDefinitions(doc),
		}
	}
}

func (b *scipGraphBuilder) registerSymbolDefinitions(documents []*scip.Document) {
	for _, doc := range documents {
		filePath := canonicalPath(doc.GetRelativePath())
		fileID := b.fileNodeByPath[filePath]
		if fileID == "" {
			continue
		}

		for _, occ := range doc.GetOccurrences() {
			if !isSCIPDefinitionRole(occ.GetSymbolRoles()) {
				continue
			}
			symbol := occ.GetSymbol()
			if symbol == "" {
				continue
			}
			startLine, endLine := scipOccurrenceLines(occ.GetRange())
			nodeType := b.nodeTypeForSymbol(symbol)
			if nodeType == "" {
				continue
			}

			nodeID := makeSCIPNodeID(symbol, filePath, startLine)
			if _, exists := b.nodeMap[nodeID]; !exists {
				b.nodeMap[nodeID] = &Node{
					ID:        nodeID,
					Type:      nodeType,
					Name:      b.displayNameForSymbol(symbol),
					FilePath:  filePath,
					StartLine: startLine,
					EndLine:   endLine,
					Language:  doc.GetLanguage(),
					Package:   scipPackageForSymbol(symbol),
					IsExport:  scip.IsGlobalSymbol(symbol),
					IsTest:    isSCIPTestFile(filePath),
				}
				b.graphData.Edges = append(b.graphData.Edges, Edge{
					Source:     fileID,
					Target:     nodeID,
					Type:       EdgeContains,
					Confidence: EdgeConfidenceExact,
				})
			}
			b.symbolNodeByID[symbol] = nodeID
		}
	}

	for symbol, nodeID := range b.symbolNodeByID {
		info := b.symbolInfoByID[symbol]
		if info == nil {
			continue
		}
		parentSymbol := info.GetEnclosingSymbol()
		if parentSymbol == "" {
			parentSymbol = scipEnclosingSymbol(symbol)
		}
		parentID := b.symbolNodeByID[parentSymbol]
		child := b.nodeMap[nodeID]
		if parentID == "" || child == nil || child.Type != NodeFunction {
			continue
		}
		parent := b.nodeMap[parentID]
		if parent == nil || parent.Type != NodeClass {
			continue
		}
		b.graphData.Edges = append(b.graphData.Edges, Edge{
			Source:     parentID,
			Target:     nodeID,
			Type:       EdgeContains,
			Confidence: EdgeConfidenceExact,
		})
	}
}

func (b *scipGraphBuilder) registerSymbolRelationships() {
	for symbol, info := range b.symbolInfoByID {
		sourceID := b.symbolNodeByID[symbol]
		source := b.nodeMap[sourceID]
		if source == nil {
			continue
		}
		for _, rel := range info.GetRelationships() {
			targetID := b.symbolNodeByID[rel.GetSymbol()]
			if targetID == "" || targetID == sourceID {
				continue
			}
			edgeType := EdgeCalls
			switch {
			case rel.GetIsImplementation():
				edgeType = EdgeImplements
			case rel.GetIsReference():
				edgeType = EdgeCalls
			case rel.GetIsTypeDefinition():
				edgeType = EdgeExtends
			default:
				continue
			}
			if source.Type == NodeClass && edgeType == EdgeCalls {
				edgeType = EdgeExtends
			}
			b.graphData.Edges = append(b.graphData.Edges, Edge{
				Source:     sourceID,
				Target:     targetID,
				Type:       edgeType,
				Confidence: EdgeConfidenceExact,
			})
		}
	}
}

func (b *scipGraphBuilder) registerReferenceEdges(documents []*scip.Document) {
	edgeSet := map[string]struct{}{}
	for _, edge := range b.graphData.Edges {
		edgeSet[edgeKey(edge.Source, edge.Target, edge.Type)] = struct{}{}
	}

	for _, doc := range documents {
		filePath := canonicalPath(doc.GetRelativePath())
		fileID := b.fileNodeByPath[filePath]
		if fileID == "" {
			continue
		}

		scopes := b.definitionScopes(doc)
		for _, occ := range doc.GetOccurrences() {
			if occ.GetSymbol() == "" || isSCIPDefinitionRole(occ.GetSymbolRoles()) || isSCIPImportRole(occ.GetSymbolRoles()) {
				continue
			}
			targetID := b.symbolNodeByID[occ.GetSymbol()]
			if targetID == "" {
				continue
			}
			sourceID := fileID
			line, _ := scipOccurrenceLines(occ.GetRange())
			if scopeID := enclosingSCIPScopeID(scopes, line, targetID); scopeID != "" {
				sourceID = scopeID
			}
			if sourceID == targetID {
				continue
			}

			key := edgeKey(sourceID, targetID, EdgeCalls)
			if _, exists := edgeSet[key]; exists {
				continue
			}
			edgeSet[key] = struct{}{}
			b.graphData.Edges = append(b.graphData.Edges, Edge{
				Source:     sourceID,
				Target:     targetID,
				Type:       EdgeCalls,
				Confidence: EdgeConfidenceExact,
			})
			b.graphData.ResolutionStats.ExactCallEdges++
		}
	}
}

func (b *scipGraphBuilder) definitionScopes(doc *scip.Document) []scipDefinitionScope {
	scopes := make([]scipDefinitionScope, 0, len(doc.GetOccurrences()))
	for _, occ := range doc.GetOccurrences() {
		if !isSCIPDefinitionRole(occ.GetSymbolRoles()) {
			continue
		}
		nodeID := b.symbolNodeByID[occ.GetSymbol()]
		node := b.nodeMap[nodeID]
		if node == nil || node.Type != NodeFunction {
			continue
		}
		startLine, endLine := scipOccurrenceLines(occ.GetRange())
		scopes = append(scopes, scipDefinitionScope{startLine: startLine, endLine: endLine, nodeID: nodeID})
	}
	sort.Slice(scopes, func(i, j int) bool {
		if scopes[i].startLine != scopes[j].startLine {
			return scopes[i].startLine > scopes[j].startLine
		}
		return scopes[i].endLine < scopes[j].endLine
	})
	return scopes
}

func (b *scipGraphBuilder) flushNodes() {
	ids := make([]string, 0, len(b.nodeMap))
	for id := range b.nodeMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	b.graphData.Nodes = make([]Node, 0, len(ids))
	for _, id := range ids {
		b.graphData.Nodes = append(b.graphData.Nodes, *b.nodeMap[id])
	}
}

func (b *scipGraphBuilder) nodeTypeForSymbol(symbol string) NodeType {
	if info := b.symbolInfoByID[symbol]; info != nil {
		switch strings.ToLower(info.GetKind().String()) {
		case "function", "method", "constructor", "macro":
			return NodeFunction
		case "class", "struct", "interface", "trait", "enum", "type", "object":
			return NodeClass
		}
	}

	switch {
	case strings.HasSuffix(symbol, "().") || strings.HasSuffix(symbol, "(#).") || strings.HasSuffix(symbol, "().") || strings.HasSuffix(symbol, "()."):
		return NodeFunction
	case strings.HasSuffix(symbol, "#") || strings.HasSuffix(symbol, ".") || strings.HasSuffix(symbol, ":"):
		return NodeClass
	}
	return NodeFunction
}

func (b *scipGraphBuilder) displayNameForSymbol(symbol string) string {
	if info := b.symbolInfoByID[symbol]; info != nil && info.GetDisplayName() != "" {
		return info.GetDisplayName()
	}
	trimmed := strings.TrimSpace(symbol)
	if trimmed == "" {
		return "symbol"
	}
	parts := strings.Fields(trimmed)
	descriptors := parts
	if len(parts) >= 4 {
		descriptors = parts[3:]
	}
	for i := len(descriptors) - 1; i >= 0; i-- {
		descriptor := descriptors[i]
		name := strings.TrimRight(descriptor, ".#:/)")
		if idx := strings.IndexByte(name, '('); idx >= 0 {
			name = name[:idx]
		}
		if name != "" {
			return name
		}
	}
	return trimmed
}

func makeSCIPNodeID(symbol, filePath string, startLine int) string {
	return "scip:" + hashStr(fmt.Sprintf("%s:%s:%d", symbol, canonicalPath(filePath), startLine))
}

func scipPackageForSymbol(symbol string) string {
	parts := strings.Fields(strings.TrimSpace(symbol))
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}

func scipEnclosingSymbol(symbol string) string {
	parts := strings.Fields(strings.TrimSpace(symbol))
	if len(parts) <= 4 {
		return ""
	}
	parent := append([]string{}, parts[:len(parts)-1]...)
	return strings.Join(parent, " ")
}

func scipOccurrenceLines(scipRange []int32) (int, int) {
	if len(scipRange) == 0 {
		return 0, 0
	}
	startLine := int(scipRange[0]) + 1
	endLine := startLine
	if len(scipRange) >= 4 {
		endLine = int(scipRange[2]) + 1
	}
	return startLine, endLine
}

func enclosingSCIPScopeID(scopes []scipDefinitionScope, line int, targetID string) string {
	for _, scope := range scopes {
		if scope.nodeID == targetID {
			continue
		}
		if line >= scope.startLine {
			return scope.nodeID
		}
	}
	return ""
}

func edgeKey(sourceID, targetID string, edgeType EdgeType) string {
	return sourceID + "|" + targetID + "|" + string(edgeType)
}

func isSCIPDefinitionRole(role int32) bool {
	return role&int32(scip.SymbolRole_Definition) != 0
}

func isSCIPImportRole(role int32) bool {
	return role&int32(scip.SymbolRole_Import) != 0
}

func isSCIPTestFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, ".test.") ||
		strings.Contains(lower, ".spec.") ||
		strings.Contains(lower, "_test.") ||
		strings.HasSuffix(lower, "_test.go") ||
		strings.HasPrefix(filepath.Base(lower), "test_")
}

func countSCIPDefinitions(doc *scip.Document) int {
	count := 0
	for _, occ := range doc.GetOccurrences() {
		if occ.GetSymbol() != "" && isSCIPDefinitionRole(occ.GetSymbolRoles()) {
			count++
		}
	}
	return count
}
