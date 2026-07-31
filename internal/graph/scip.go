package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anurag/tracescope/internal/parser"
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
		graphData:         &GraphData{IndexSource: "scip", FileMetadata: map[string]*FileMetadata{}},
		nodeMap:           map[string]*Node{},
		fileNodeByPath:    map[string]string{},
		packageByFilePath: map[string]string{},
		packageFileIDs:    map[string][]string{},
		sourceLinesByPath: map[string][]string{},
		symbolNodeByID:    map[string]string{},
		symbolInfoByID:    map[string]*scip.SymbolInformation{},
		edgeSet:           map[string]struct{}{},
	}
	builder.loadSourceLines(indexPaths, documents)
	builder.collectSymbolInfo(documents, externalSymbols)
	builder.registerDocuments(documents)
	builder.registerSymbolDefinitions(documents)
	builder.refineGoFunctionBounds()
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
	graphData         *GraphData
	nodeMap           map[string]*Node
	fileNodeByPath    map[string]string
	packageByFilePath map[string]string
	packageFileIDs    map[string][]string
	sourceLinesByPath map[string][]string
	symbolNodeByID    map[string]string
	symbolInfoByID    map[string]*scip.SymbolInformation
	edgeSet           map[string]struct{}
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

func (b *scipGraphBuilder) loadSourceLines(indexPaths []string, documents []*scip.Document) {
	roots := map[string]bool{}
	for _, indexPath := range indexPaths {
		roots[sourceRootForSCIPIndex(indexPath)] = true
	}

	for _, doc := range documents {
		relPath := canonicalPath(doc.GetRelativePath())
		if relPath == "" || b.sourceLinesByPath[relPath] != nil {
			continue
		}
		for root := range roots {
			sourcePath := filepath.Join(root, filepath.FromSlash(relPath))
			source, err := os.ReadFile(sourcePath)
			if err != nil {
				continue
			}
			b.sourceLinesByPath[relPath] = strings.Split(string(source), "\n")
			break
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
			// scip-go (>= v0.2.1) and scip-typescript provide the full
			// definition body via the enclosing range; prefer its end line so
			// a body-only diff change still overlaps the function node.
			if enclosing := occ.GetEnclosingRange(); len(enclosing) > 0 {
				if _, enclosingEnd := scipOccurrenceLines(enclosing); enclosingEnd > endLine {
					endLine = enclosingEnd
				}
			}
			nodeType := b.nodeTypeForSymbol(symbol, filePath, startLine)
			if nodeType == "" {
				continue
			}
			if pkg := scipPackageForSymbol(symbol); pkg != "" {
				if b.packageByFilePath[filePath] == "" {
					b.packageByFilePath[filePath] = pkg
				}
				if !isSCIPTestFile(filePath) {
					b.packageFileIDs[pkg] = appendUnique(b.packageFileIDs[pkg], fileID)
				}
			}

			nodeID := makeSCIPNodeID(symbol, filePath)
			if node, exists := b.nodeMap[nodeID]; exists {
				if startLine > 0 && (node.StartLine == 0 || startLine < node.StartLine) {
					node.StartLine = startLine
				}
				if endLine > node.EndLine {
					node.EndLine = endLine
				}
			} else {
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

	// Iterate in a stable order so class→method CONTAINS edges are emitted
	// deterministically (Go map iteration order is randomized).
	symbols := make([]string, 0, len(b.symbolNodeByID))
	for symbol := range b.symbolNodeByID {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	for _, symbol := range symbols {
		nodeID := b.symbolNodeByID[symbol]
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

// refineGoFunctionBounds widens Go function nodes to their real body end line.
// scip-go emits identifier-only definition ranges (EndLine == StartLine), which
// would let a body-only diff change miss the function and a file-scope
// reference past the function be misattributed to it. The native Go parser
// derives exact body bounds from go/ast, so re-parse each Go source file and
// adopt those end lines.
func (b *scipGraphBuilder) refineGoFunctionBounds() {
	funcsByFile := map[string][]*Node{}
	for _, node := range b.nodeMap {
		if node.Type == NodeFunction && strings.HasSuffix(strings.ToLower(node.FilePath), ".go") {
			funcsByFile[node.FilePath] = append(funcsByFile[node.FilePath], node)
		}
	}

	goParser := parser.NewGoParser()
	for filePath, funcs := range funcsByFile {
		lines := b.sourceLinesByPath[canonicalPath(filePath)]
		if len(lines) == 0 {
			continue
		}
		result, _ := goParser.Parse(filePath, []byte(strings.Join(lines, "\n")))
		if result == nil {
			continue
		}
		endByStartLine := make(map[int]int, len(result.Functions))
		for _, fn := range result.Functions {
			if fn.EndLine > endByStartLine[fn.StartLine] {
				endByStartLine[fn.StartLine] = fn.EndLine
			}
		}
		for _, node := range funcs {
			if end, ok := endByStartLine[node.StartLine]; ok && end > node.EndLine {
				node.EndLine = end
			}
		}
	}
}

func (b *scipGraphBuilder) registerSymbolRelationships() {
	// Iterate in a stable order so EXTENDS/IMPLEMENTS edges are emitted
	// deterministically (Go map iteration order is randomized).
	symbols := make([]string, 0, len(b.symbolInfoByID))
	for symbol := range b.symbolInfoByID {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	for _, symbol := range symbols {
		info := b.symbolInfoByID[symbol]
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
			if occ.GetSymbol() == "" || isSCIPDefinitionRole(occ.GetSymbolRoles()) {
				continue
			}
			targetID := b.symbolNodeByID[occ.GetSymbol()]
			if isSCIPImportRole(occ.GetSymbolRoles()) {
				targetFileID := ""
				if targetID != "" {
					if target := b.nodeMap[targetID]; target != nil {
						targetFileID = b.fileNodeByPath[canonicalPath(target.FilePath)]
					}
				}
				if targetFileID == "" {
					targetFileID = b.packageImportFileID(occ.GetSymbol())
				}
				if targetFileID == "" || targetFileID == fileID {
					continue
				}
				b.addEdge(fileID, targetFileID, EdgeImports, EdgeConfidenceExact)
				continue
			}
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

			target := b.nodeMap[targetID]
			targetFileID := ""
			if target != nil {
				targetFileID = b.packageFileIDForPackage(target.Package)
				if targetFileID == "" {
					targetFileID = b.fileNodeByPath[canonicalPath(target.FilePath)]
				}
			}
			if target != nil &&
				targetFileID != "" &&
				targetFileID != fileID &&
				b.packageByFilePath[filePath] != "" &&
				target.Package != "" &&
				b.packageByFilePath[filePath] != target.Package {
				b.addEdge(fileID, targetFileID, EdgeImports, EdgeConfidenceExact)
			}
		}
	}
}

func (b *scipGraphBuilder) packageImportFileID(symbol string) string {
	return b.packageFileIDForPackage(scipPackageForSymbol(symbol))
}

func (b *scipGraphBuilder) packageFileIDForPackage(pkg string) string {
	ids := b.packageFileIDs[pkg]
	if len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)
	return ids[0]
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
		// Prefer the enclosing range (the full definition body) when the indexer
		// provides it — scip-typescript does, scip-go does not. Without it,
		// startLine == endLine (the identifier only) and enclosingSCIPScopeID
		// falls back to a nearest-preceding-definition heuristic.
		if enclosing := occ.GetEnclosingRange(); len(enclosing) > 0 {
			startLine, endLine = scipOccurrenceLines(enclosing)
		}
		// A refined node range (Go function bounds derived from the native
		// parser) is more accurate than an identifier-only occurrence range.
		if node.EndLine > endLine {
			startLine, endLine = node.StartLine, node.EndLine
		}
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

func (b *scipGraphBuilder) nodeTypeForSymbol(symbol, filePath string, line int) NodeType {
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
		if !b.hasCallableSourceDeclaration(filePath, line, shortSCIPDescriptorName(descriptor)) {
			return ""
		}
		return NodeFunction
	case strings.HasSuffix(descriptor, "#") || strings.HasSuffix(descriptor, ":"):
		return NodeClass
	}
	return ""
}

func (b *scipGraphBuilder) hasCallableSourceDeclaration(filePath string, line int, name string) bool {
	if line <= 0 || name == "" {
		return true
	}
	lines := b.sourceLinesByPath[canonicalPath(filePath)]
	if len(lines) == 0 || line > len(lines) {
		return true
	}
	text := lines[line-1]
	if strings.Contains(text, name+"(") {
		return true
	}
	return false
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

func makeSCIPNodeID(symbol, filePath string) string {
	return "scip:" + hashStr(fmt.Sprintf("%s:%s", symbol, canonicalPath(filePath)))
}

func scipPackageForSymbol(symbol string) string {
	parts := strings.Fields(strings.TrimSpace(symbol))
	if len(parts) < 3 {
		return ""
	}
	if len(parts) >= 5 {
		for _, descriptor := range parts[4:] {
			if pkg := scipPackageDescriptorPath(descriptor); pkg != "" {
				return pkg
			}
		}
	}
	return parts[2]
}

func scipPackageDescriptorPath(descriptor string) string {
	descriptor = strings.TrimSpace(descriptor)
	if !strings.HasPrefix(descriptor, "`") {
		return ""
	}
	end := strings.Index(descriptor[1:], "`")
	if end < 0 {
		return ""
	}
	pkg := descriptor[1 : end+1]
	return strings.TrimSuffix(pkg, "/")
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
		if line < scope.startLine {
			continue
		}
		// When a real multi-line body range is known (endLine > startLine), the
		// reference must fall inside it. With identifier-only ranges
		// (endLine == startLine) fall back to nearest-preceding-definition.
		if scope.endLine > scope.startLine && line > scope.endLine {
			continue
		}
		return scope.nodeID
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
	if strings.Contains(path, ":/") {
		return false
	}
	lower := strings.ToLower(path)
	for _, segment := range []string{".next", "node_modules", "dist", "build", "coverage"} {
		if lower == segment || strings.HasPrefix(lower, segment+"/") || strings.Contains(lower, "/"+segment+"/") {
			return false
		}
	}
	return true
}

func sourceRootForSCIPIndex(indexPath string) string {
	dir := filepath.Dir(indexPath)
	if filepath.Base(dir) == "scip" && filepath.Base(filepath.Dir(dir)) == ".tracescope" {
		return filepath.Dir(filepath.Dir(dir))
	}
	return dir
}
