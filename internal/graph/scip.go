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

// BuildFromSCIP loads one index.scip file and converts it into GraphData.
func BuildFromSCIP(indexPath string) (*GraphData, error) {
	return BuildFromSCIPFiles([]string{indexPath})
}

// BuildFromSCIPFiles loads one or more SCIP index files and merges them into
// one GraphData instance.
func BuildFromSCIPFiles(indexPaths []string) (*GraphData, error) {
	var documents []*scip.Document
	var externalSymbols []*scip.SymbolInformation

	for _, indexPath := range indexPaths {
		index, err := loadSCIPIndex(indexPath)
		if err != nil {
			return nil, err
		}
		for _, doc := range index.GetDocuments() {
			if !isProjectSCIPDocument(doc.GetRelativePath()) {
				continue
			}
			documents = append(documents, doc)
		}
		externalSymbols = append(externalSymbols, index.GetExternalSymbols()...)
	}

	builder := scipGraphBuilder{
		graphData:      &GraphData{IndexSource: "scip", FileMetadata: map[string]*FileMetadata{}},
		nodeMap:        map[string]*Node{},
		fileNodeByPath: map[string]string{},
		symbolNodeByID: map[string]string{},
		symbolInfoByID: map[string]*scip.SymbolInformation{},
		edgeSet:        map[string]struct{}{},
	}
	builder.collectSymbolInfo(documents, externalSymbols)
	builder.registerDocuments(documents)
	builder.registerSymbolDefinitions(documents)
	builder.registerSymbolRelationships()
	builder.registerReferenceEdges(documents)
	builder.flushNodes()
	return builder.graphData, nil
}

func loadSCIPIndex(indexPath string) (*scip.Index, error) {
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("reading SCIP index: %w", err)
	}

	var index scip.Index
	if err := proto.Unmarshal(raw, &index); err != nil {
		return nil, fmt.Errorf("decoding SCIP index: %w", err)
	}
	return &index, nil
}

type scipGraphBuilder struct {
	graphData      *GraphData
	nodeMap        map[string]*Node
	fileNodeByPath map[string]string
	symbolNodeByID map[string]string
	symbolInfoByID map[string]*scip.SymbolInformation
	edgeSet        map[string]struct{}
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
				b.addEdge(fileID, nodeID, EdgeContains, EdgeConfidenceExact)
			}
			b.symbolNodeByID[symbol] = nodeID
		}
	}

	for symbol, nodeID := range b.symbolNodeByID {
		var parentSymbol string
		if info := b.symbolInfoByID[symbol]; info != nil {
			parentSymbol = info.GetEnclosingSymbol()
		}
		parentID := b.symbolNodeByID[parentSymbol]
		if parentID == "" {
			parentSymbol = scipEnclosingSymbol(symbol)
			parentID = b.symbolNodeByID[parentSymbol]
		}
		child := b.nodeMap[nodeID]
		if parentID == "" || child == nil || child.Type != NodeFunction {
			continue
		}
		parent := b.nodeMap[parentID]
		if parent == nil || parent.Type != NodeClass {
			continue
		}
		b.addEdge(parentID, nodeID, EdgeContains, EdgeConfidenceExact)
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
			target := b.nodeMap[targetID]
			edgeType, ok := scipRelationshipEdgeType(source, target, rel)
			if !ok {
				continue
			}
			b.addEdge(sourceID, targetID, edgeType, EdgeConfidenceExact)
		}
	}
}

func (b *scipGraphBuilder) registerReferenceEdges(documents []*scip.Document) {
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
			b.addEdge(sourceID, targetID, EdgeCalls, EdgeConfidenceExact)
		}
	}
}

func (b *scipGraphBuilder) addEdge(sourceID, targetID string, edgeType EdgeType, confidence EdgeConfidence) {
	key := edgeKey(sourceID, targetID, edgeType)
	if _, exists := b.edgeSet[key]; exists {
		return
	}
	b.edgeSet[key] = struct{}{}
	b.graphData.Edges = append(b.graphData.Edges, Edge{
		Source:     sourceID,
		Target:     targetID,
		Type:       edgeType,
		Confidence: confidence,
	})
	if confidence != EdgeConfidenceExact {
		return
	}
	switch edgeType {
	case EdgeCalls:
		b.graphData.ResolutionStats.ExactCallEdges++
	case EdgeExtends, EdgeImplements:
		b.graphData.ResolutionStats.ExactInheritance++
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
		case "field", "property", "variable", "constant", "parameter", "local", "package", "module", "namespace":
			return ""
		}
	}

	descriptor := symbol
	if parts := strings.Fields(strings.TrimSpace(symbol)); len(parts) > 0 {
		descriptor = parts[len(parts)-1]
	}
	switch {
	case strings.Contains(descriptor, "("):
		return NodeFunction
	case strings.Contains(descriptor, "#") && strings.HasSuffix(descriptor, "."):
		return NodeFunction
	case strings.HasSuffix(descriptor, "#") || strings.HasSuffix(descriptor, ":"):
		return NodeClass
	}
	return ""
}

func (b *scipGraphBuilder) displayNameForSymbol(symbol string) string {
	if info := b.symbolInfoByID[symbol]; info != nil && info.GetDisplayName() != "" {
		if name := shortSCIPDescriptorName(info.GetDisplayName()); name != "" {
			return name
		}
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
		if name := shortSCIPDescriptorName(descriptors[i]); name != "" {
			return name
		}
	}
	return trimmed
}

func shortSCIPDescriptorName(descriptor string) string {
	name := strings.TrimSpace(descriptor)
	if name == "" {
		return ""
	}
	if idx := strings.LastIndexByte(name, '/'); idx >= 0 && idx+1 < len(name) {
		name = name[idx+1:]
	}
	name = strings.Trim(name, "`")
	if strings.HasPrefix(name, "(*") || strings.HasPrefix(name, "(") {
		if idx := strings.Index(name, ")."); idx >= 0 && idx+2 < len(name) {
			name = name[idx+2:]
		}
	}
	if idx := strings.LastIndexByte(name, '#'); idx >= 0 {
		if idx+1 < len(name) {
			name = name[idx+1:]
		} else {
			name = name[:idx]
		}
	}
	name = strings.TrimRight(name, ".#:/)")
	if idx := strings.IndexByte(name, '('); idx >= 0 {
		name = name[:idx]
	}
	if idx := strings.IndexByte(name, '['); idx >= 0 {
		name = name[:idx]
	}
	return strings.Trim(name, "`")
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
	if len(parts) < 4 {
		return ""
	}
	if parentDescriptor := scipParentDescriptor(parts[len(parts)-1]); parentDescriptor != "" {
		parent := append([]string{}, parts[:len(parts)-1]...)
		parent = append(parent, parentDescriptor)
		return strings.Join(parent, " ")
	}
	if len(parts) == 4 {
		return ""
	}
	parent := append([]string{}, parts[:len(parts)-1]...)
	return strings.Join(parent, " ")
}

func scipParentDescriptor(descriptor string) string {
	descriptor = strings.TrimSpace(descriptor)
	switch {
	case descriptor == "":
		return ""
	case strings.HasPrefix(descriptor, "(*") && strings.Contains(descriptor, ")."):
		receiver := strings.TrimPrefix(descriptor, "(*")
		receiver = receiver[:strings.Index(receiver, ").")]
		receiver = strings.TrimPrefix(receiver, "*")
		if receiver == "" {
			return ""
		}
		return receiver + "#"
	case strings.HasPrefix(descriptor, "(") && strings.Contains(descriptor, ")."):
		receiver := strings.TrimPrefix(descriptor, "(")
		receiver = receiver[:strings.Index(receiver, ").")]
		receiver = strings.TrimPrefix(receiver, "*")
		if receiver == "" {
			return ""
		}
		return receiver + "#"
	case strings.Contains(descriptor, "#") && strings.Contains(descriptor, "("):
		hash := strings.IndexByte(descriptor, '#')
		if hash <= 0 {
			return ""
		}
		return descriptor[:hash+1]
	case strings.Contains(descriptor, "#") && strings.HasSuffix(descriptor, "."):
		hash := strings.IndexByte(descriptor, '#')
		if hash <= 0 {
			return ""
		}
		return descriptor[:hash+1]
	default:
		return ""
	}
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

func scipRelationshipEdgeType(source, target *Node, rel *scip.Relationship) (EdgeType, bool) {
	if source == nil || target == nil || rel == nil {
		return "", false
	}
	switch {
	case rel.GetIsImplementation() && source.Type == NodeClass && target.Type == NodeClass:
		return EdgeImplements, true
	case rel.GetIsTypeDefinition() && source.Type == NodeClass && target.Type == NodeClass:
		return EdgeExtends, true
	case rel.GetIsReference() && source.Type == NodeClass && target.Type == NodeClass:
		return EdgeExtends, true
	case rel.GetIsReference() && source.Type == NodeFunction && target.Type == NodeFunction:
		return EdgeCalls, true
	case rel.GetIsReference() && source.Type == NodeFunction && target.Type == NodeClass:
		return EdgeCalls, true
	default:
		return "", false
	}
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

func isProjectSCIPDocument(relativePath string) bool {
	path := canonicalPath(relativePath)
	if path == "" || path == "." || strings.HasPrefix(path, "../") || strings.HasPrefix(path, "/") {
		return false
	}
	return !strings.Contains(path, ":/")
}
