package graph

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anurag/tracescope/internal/parser"
	"github.com/rs/zerolog/log"
)

// Builder constructs the dependency graph from parsed file results.
type Builder struct{}

// NewBuilder creates a new graph builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// Build performs a 3-pass build of the dependency graph:
// 1. Register all definitions (File, Function, Class nodes + CONTAINS edges)
// 2. Resolve imports (IMPORTS edges)
// 3. Resolve calls (CALLS edges, matching call targets to definitions cross-file)
func (b *Builder) Build(results []*parser.FileResult) *GraphData {
	gd := &GraphData{}

	// Data structures for cross-referencing
	funcByName := make(map[string][]string)
	funcByQualified := make(map[string][]string)
	fileNodeByPath := make(map[string]string)
	importMap := make(map[string]map[string]string)
	fileByDir := make(map[string][]*parser.FileResult)
	classByName := make(map[string][]string)

	// Use heap-allocated nodes to avoid dangling pointers from slice growth
	nodeMap := make(map[string]*Node)

	// ── Pass 1: Register definitions ──

	for _, fr := range results {
		// File node
		fileID := makeFileID(fr.FilePath)
		fileNode := &Node{
			ID:       fileID,
			Type:     NodeFile,
			Name:     filepath.Base(fr.FilePath),
			FilePath: fr.FilePath,
			Language: string(fr.Language),
			Package:  fr.Package,
			IsTest:   fr.IsTestFile,
		}
		nodeMap[fileID] = fileNode
		fileNodeByPath[fr.FilePath] = fileID
		fileNodeByPath[canonicalPath(fr.FilePath)] = fileID

		dir := filepath.Dir(fr.FilePath)
		fileByDir[dir] = append(fileByDir[dir], fr)

		// Function nodes
		for _, fn := range fr.Functions {
			funcID := makeFuncID(fr.FilePath, fn.Name, fn.Receiver, fn.StartLine)
			funcNode := &Node{
				ID:        funcID,
				Type:      NodeFunction,
				Name:      fn.Name,
				FilePath:  fr.FilePath,
				StartLine: fn.StartLine,
				EndLine:   fn.EndLine,
				Language:  string(fr.Language),
				Package:   fr.Package,
				IsExport:  fn.IsExport,
				IsTest:    fr.IsTestFile,
				IsInit:    fn.IsInit,
			}
			nodeMap[funcID] = funcNode

			// CONTAINS edge: file → function
			gd.Edges = append(gd.Edges, Edge{
				Source:     fileID,
				Target:     funcID,
				Type:       EdgeContains,
				Confidence: EdgeConfidenceExact,
			})

			// Register in lookup maps
			funcByName[fn.Name] = append(funcByName[fn.Name], funcID)

			// Qualified names for Go
			if fr.Language == parser.LangGo && fr.Package != "" {
				if fn.Receiver != "" {
					key := fr.Package + "." + fn.Receiver + "." + fn.Name
					funcByQualified[key] = appendUnique(funcByQualified[key], funcID)
				}
				key := fr.Package + "." + fn.Name
				funcByQualified[key] = appendUnique(funcByQualified[key], funcID)
			}

			// For JS/TS/Python, index both basename and full normalized file
			// qualifier. Basename keys are ambiguous across folders, so call
			// resolution only uses them when they map to a single target.
			if fr.Language != parser.LangGo {
				keys := []string{
					fileBaseName(fr.FilePath) + "." + fn.Name,
					fileQualifier(fr.FilePath) + "." + fn.Name,
				}
				if fn.Receiver != "" {
					keys = append(keys, fn.Receiver+"."+fn.Name)
					keys = append(keys, fileQualifier(fr.FilePath)+"."+fn.Receiver+"."+fn.Name)
				}
				for _, key := range keys {
					funcByQualified[key] = appendUnique(funcByQualified[key], funcID)
				}
			}
		}

		// Class nodes
		for _, cls := range fr.Classes {
			classID := makeClassID(fr.FilePath, cls.Name)
			classNode := &Node{
				ID:        classID,
				Type:      NodeClass,
				Name:      cls.Name,
				FilePath:  fr.FilePath,
				StartLine: cls.StartLine,
				EndLine:   cls.EndLine,
				Language:  string(fr.Language),
				Package:   fr.Package,
				IsExport:  cls.IsExport,
				IsTest:    fr.IsTestFile,
			}
			nodeMap[classID] = classNode
			classByName[cls.Name] = appendUnique(classByName[cls.Name], classID)

			// CONTAINS edge: file → class
			gd.Edges = append(gd.Edges, Edge{
				Source:     fileID,
				Target:     classID,
				Type:       EdgeContains,
				Confidence: EdgeConfidenceExact,
			})
		}
	}

	// ── Pass 1.5: Class→Method CONTAINS edges + EXTENDS/IMPLEMENTS edges ──

	// Build class lookup by name (scoped by package for Go, by file for others)
	classByPkgName := make(map[string]string)  // "pkg.ClassName" → classID
	classByFileName := make(map[string]string) // "filePath:ClassName" → classID
	for _, fr := range results {
		for _, cls := range fr.Classes {
			classID := makeClassID(fr.FilePath, cls.Name)
			if fr.Language == parser.LangGo && fr.Package != "" {
				classByPkgName[fr.Package+"."+cls.Name] = classID
			}
			classByFileName[fr.FilePath+":"+cls.Name] = classID
		}
	}

	for _, fr := range results {
		// Class→Method CONTAINS edges
		classIDs := make(map[string]string) // className → classID (within this file's package)
		for _, cls := range fr.Classes {
			classIDs[cls.Name] = makeClassID(fr.FilePath, cls.Name)
		}
		// Also check other files in same Go package
		if fr.Language == parser.LangGo {
			dir := filepath.Dir(fr.FilePath)
			for _, other := range fileByDir[dir] {
				if other.Language == parser.LangGo && other.Package == fr.Package {
					for _, cls := range other.Classes {
						if _, exists := classIDs[cls.Name]; !exists {
							classIDs[cls.Name] = makeClassID(other.FilePath, cls.Name)
						}
					}
				}
			}
		}
		for _, fn := range fr.Functions {
			if fn.Receiver != "" {
				if classID, ok := classIDs[fn.Receiver]; ok {
					funcID := makeFuncID(fr.FilePath, fn.Name, fn.Receiver, fn.StartLine)
					gd.Edges = append(gd.Edges, Edge{
						Source:     classID,
						Target:     funcID,
						Type:       EdgeContains,
						Confidence: EdgeConfidenceExact,
					})
				}
			}
		}

		// EXTENDS edges from Bases
		for _, cls := range fr.Classes {
			classID := makeClassID(fr.FilePath, cls.Name)
			for _, baseName := range cls.Bases {
				targetID := ""
				confidence := EdgeConfidenceExact
				// Try same-file first
				if id, ok := classByFileName[fr.FilePath+":"+baseName]; ok {
					targetID = id
				}
				// Try same-package (Go)
				if targetID == "" && fr.Language == parser.LangGo && fr.Package != "" {
					if id, ok := classByPkgName[fr.Package+"."+baseName]; ok {
						targetID = id
					}
				}
				// Try any file (cross-file JS/TS/Python)
				if targetID == "" {
					if id, status := resolveUnique(classByName[baseName]); id != "" {
						targetID = id
						confidence = EdgeConfidenceHeuristic
					} else if status == resolutionAmbiguous {
						gd.ResolutionStats.AmbiguousInheritance++
					} else {
						gd.ResolutionStats.UnresolvedInheritance++
					}
				}
				if targetID == "" {
					continue
				}
				if targetID != "" && targetID != classID {
					edgeType := EdgeExtends
					if cls.Kind == "interface" || cls.Kind == "struct" {
						edgeType = EdgeExtends
					}
					gd.Edges = append(gd.Edges, Edge{
						Source:     classID,
						Target:     targetID,
						Type:       edgeType,
						Confidence: confidence,
					})
					if confidence == EdgeConfidenceHeuristic {
						gd.ResolutionStats.HeuristicInheritance++
					} else {
						gd.ResolutionStats.ExactInheritance++
					}
				}
			}
		}
	}

	// ── Pass 2: Resolve imports ──

	for _, fr := range results {
		fileID := fileNodeByPath[fr.FilePath]
		fileImportMap := make(map[string]string)

		for _, imp := range fr.Imports {
			// Try to find the target file
			targetFileID := b.resolveImport(fr, imp, fileNodeByPath, fileByDir, results)
			if targetFileID != "" {
				gd.Edges = append(gd.Edges, Edge{
					Source:     fileID,
					Target:     targetFileID,
					Type:       EdgeImports,
					Confidence: EdgeConfidenceExact,
				})

				// Build alias → target file mapping
				alias := imp.Alias
				if alias == "" {
					alias = importBaseName(imp.Path, fr.Language)
				}
				if alias != "" {
					fileImportMap[alias] = targetFileID
				}
			}
		}

		importMap[fr.FilePath] = fileImportMap
	}

	// ── Pass 3: Resolve calls ──

	for _, fr := range results {
		for _, call := range fr.Calls {
			targetID, confidence, status := b.resolveCall(fr, call, funcByName, funcByQualified, importMap, fileNodeByPath, nodeMap)
			if targetID == "" {
				switch status {
				case resolutionAmbiguous:
					gd.ResolutionStats.AmbiguousCalls++
				case resolutionUnresolved:
					gd.ResolutionStats.UnresolvedCalls++
				}
				continue
			}

			// Find the calling function (the function that contains this call line)
			callerID := b.findContainingFunction(fr, call.Line, fileNodeByPath[fr.FilePath], gd)
			if callerID == "" {
				// If we can't find the containing function, use the file
				callerID = fileNodeByPath[fr.FilePath]
			}

			gd.Edges = append(gd.Edges, Edge{
				Source:     callerID,
				Target:     targetID,
				Type:       EdgeCalls,
				Confidence: confidence,
			})
			if confidence == EdgeConfidenceHeuristic {
				gd.ResolutionStats.HeuristicCallEdges++
			} else {
				gd.ResolutionStats.ExactCallEdges++
			}
		}
	}

	// Convert nodeMap to slice in deterministic order (sorted by ID)
	ids := make([]string, 0, len(nodeMap))
	for id := range nodeMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	gd.Nodes = make([]Node, 0, len(nodeMap))
	for _, id := range ids {
		gd.Nodes = append(gd.Nodes, *nodeMap[id])
	}

	// ── Populate FileMetadata ──

	gd.FileMetadata = make(map[string]*FileMetadata, len(results))
	for _, fr := range results {
		if fr.ContentHash != "" {
			gd.FileMetadata[fr.FilePath] = &FileMetadata{
				Hash:      fr.ContentHash,
				Language:  string(fr.Language),
				ParsedAt:  time.Now().Unix(),
				NodeCount: len(fr.Functions) + len(fr.Classes),
			}
		}
	}

	log.Debug().
		Int("nodes", len(gd.Nodes)).
		Int("edges", len(gd.Edges)).
		Msg("graph built")

	return gd
}

// resolveImport tries to find the target file node for an import.
func (b *Builder) resolveImport(fr *parser.FileResult, imp parser.Import, fileNodeByPath map[string]string, fileByDir map[string][]*parser.FileResult, results []*parser.FileResult) string {
	switch fr.Language {
	case parser.LangGo:
		return b.resolveGoImport(imp, fileNodeByPath, results)
	case parser.LangJavaScript, parser.LangTypeScript:
		return b.resolveJSImport(fr.FilePath, imp, fileNodeByPath)
	case parser.LangPython:
		return b.resolvePythonImport(fr.FilePath, imp, fileNodeByPath)
	}
	return ""
}

func (b *Builder) resolveGoImport(imp parser.Import, fileNodeByPath map[string]string, results []*parser.FileResult) string {
	// Go imports are package paths while graph nodes are file paths. Match each
	// candidate's directory against the longest package-path suffix. If exactly
	// one package directory wins, return a deterministic representative file.
	bestDirs := make(map[string][]string)
	bestScore := 0
	importPath := strings.Trim(imp.Path, "/")
	importParts := strings.Split(importPath, "/")
	for _, r := range results {
		if r.Language != parser.LangGo {
			continue
		}
		dirPath := filepath.ToSlash(filepath.Dir(r.FilePath))
		score := longestImportSuffixMatch(dirPath, importParts)
		if score == 0 {
			continue
		}
		if id, ok := fileNodeByPath[r.FilePath]; ok {
			switch {
			case score > bestScore:
				bestScore = score
				bestDirs = map[string][]string{dirPath: {id}}
			case score == bestScore:
				bestDirs[dirPath] = appendUnique(bestDirs[dirPath], id)
			}
		}
	}
	if len(bestDirs) != 1 {
		return ""
	}
	for _, ids := range bestDirs {
		sort.Strings(ids)
		return ids[0]
	}
	return ""
}

func (b *Builder) resolveJSImport(currentFile string, imp parser.Import, fileNodeByPath map[string]string) string {
	if !strings.HasPrefix(imp.Path, ".") {
		return "" // external package, skip
	}

	dir := filepath.Dir(currentFile)
	resolved := filepath.Join(dir, imp.Path)
	resolved = filepath.Clean(resolved)

	// Try with various extensions
	extensions := []string{"", ".ts", ".tsx", ".js", ".jsx"}
	for _, ext := range extensions {
		candidate := canonicalPath(resolved + ext)
		if id, ok := fileNodeByPath[candidate]; ok {
			return id
		}
	}

	// Try index files
	indexFiles := []string{"index.ts", "index.tsx", "index.js", "index.jsx"}
	for _, idx := range indexFiles {
		candidate := canonicalPath(filepath.Join(resolved, idx))
		if id, ok := fileNodeByPath[candidate]; ok {
			return id
		}
	}

	return ""
}

func (b *Builder) resolvePythonImport(currentFile string, imp parser.Import, fileNodeByPath map[string]string) string {
	path := imp.Path
	dir := filepath.Dir(currentFile)

	// Handle relative imports (leading dots)
	dots := 0
	for _, c := range path {
		if c == '.' {
			dots++
		} else {
			break
		}
	}

	if dots > 0 {
		path = path[dots:]
		// Go up (dots-1) directories: one dot = current package
		for i := 1; i < dots; i++ {
			dir = filepath.Dir(dir)
		}
	}

	// Convert dotted path to file path
	var parts []string
	if path != "" {
		parts = strings.Split(path, ".")
	}

	// For "from . import foo" (dots only, no module), resolve to __init__.py in dir
	if len(parts) == 0 && dots > 0 {
		candidate := canonicalPath(filepath.Join(dir, "__init__.py"))
		if id, ok := fileNodeByPath[candidate]; ok {
			return id
		}
		return ""
	}

	relPath := filepath.Join(append([]string{dir}, parts...)...)
	candidates := []string{
		canonicalPath(relPath + ".py"),
		canonicalPath(filepath.Join(relPath, "__init__.py")),
	}
	for _, c := range candidates {
		if id, ok := fileNodeByPath[c]; ok {
			return id
		}
	}

	// For absolute imports, use path-segment matching
	if dots == 0 && len(parts) > 0 {
		var matches []string
		for knownPath, id := range fileNodeByPath {
			if !strings.HasSuffix(knownPath, ".py") {
				continue
			}
			if pythonPathMatch(knownPath, parts) {
				matches = appendUnique(matches, id)
			}
		}
		sort.Strings(matches)
		return uniqueID(matches)
	}

	return ""
}

// pythonPathMatch checks if a file path matches the Python import parts.
// e.g., parts=["foo", "bar"] matches "src/foo/bar.py" or "src/foo/bar/__init__.py"
func pythonPathMatch(filePath string, parts []string) bool {
	normPath := filepath.ToSlash(filePath)
	segments := strings.Split(normPath, "/")

	// Try matching "parts[-1].py" as the file name
	if len(parts) == 0 {
		return false
	}
	lastPart := parts[len(parts)-1] + ".py"
	if segments[len(segments)-1] != lastPart {
		// Also try __init__.py in a matching directory
		if segments[len(segments)-1] != "__init__.py" {
			return false
		}
		if len(segments) < 2 || segments[len(segments)-2] != parts[len(parts)-1] {
			return false
		}
		// Shift: check remaining parts against directory segments
		segments = segments[:len(segments)-1]
	}

	// Check remaining import parts match path segments from right
	if len(parts) > len(segments) {
		return false
	}
	for i := 0; i < len(parts)-1; i++ {
		segIdx := len(segments) - 1 - (len(parts) - 1 - i)
		if segIdx < 0 || segments[segIdx] != parts[i] {
			return false
		}
	}
	return true
}

// resolveCall tries to find the target function node for a call expression.
type resolutionStatus int

const (
	resolutionResolved resolutionStatus = iota
	resolutionAmbiguous
	resolutionUnresolved
)

func (b *Builder) resolveCall(fr *parser.FileResult, call parser.FunctionCall, funcByName map[string][]string, funcByQualified map[string][]string, importMap map[string]map[string]string, fileNodeByPath map[string]string, nodeMap map[string]*Node) (string, EdgeConfidence, resolutionStatus) {
	// If call has a receiver (e.g., pkg.Func, obj.Method)
	if call.Receiver != "" {
		if fr.Language == parser.LangGo && call.ReceiverType != "" {
			pkgName := call.ReceiverPackage
			if pkgName == "" {
				pkgName = fr.Package
			}
			if pkgName != "" {
				qualKey := pkgName + "." + call.ReceiverType + "." + call.Name
				if id, status := resolveUnique(funcByQualified[qualKey]); id != "" {
					return id, EdgeConfidenceExact, resolutionResolved
				} else if status == resolutionAmbiguous {
					return "", "", resolutionAmbiguous
				}
			}
			typeKey := call.ReceiverType + "." + call.Name
			if id, status := resolveUnique(funcByQualified[typeKey]); id != "" {
				return id, EdgeConfidenceExact, resolutionResolved
			} else if status == resolutionAmbiguous {
				return "", "", resolutionAmbiguous
			}
		}

		if fileImports, ok := importMap[fr.FilePath]; ok {
			if targetFileID, ok := fileImports[call.Receiver]; ok {
				if targetNode, ok := nodeMap[targetFileID]; ok {
					switch fr.Language {
					case parser.LangGo:
						qualKey := targetNode.Package + "." + call.Name
						if id, status := resolveUnique(funcByQualified[qualKey]); id != "" {
							return id, EdgeConfidenceExact, resolutionResolved
						} else if status == resolutionAmbiguous {
							return "", "", resolutionAmbiguous
						}
					case parser.LangJavaScript, parser.LangTypeScript:
						qualKey := fileQualifier(targetNode.FilePath) + "." + call.Name
						if id, status := resolveUnique(funcByQualified[qualKey]); id != "" {
							return id, EdgeConfidenceExact, resolutionResolved
						} else if status == resolutionAmbiguous {
							return "", "", resolutionAmbiguous
						}
					}
				}
			}
		}

		// Fall back to receiver.name for method-like lookups. Treat this as
		// heuristic when no static receiver type was inferred.
		key := call.Receiver + "." + call.Name
		if id, status := resolveUnique(funcByQualified[key]); id != "" {
			return id, EdgeConfidenceHeuristic, resolutionResolved
		} else if status == resolutionAmbiguous {
			return "", "", resolutionAmbiguous
		}
	}

	// Simple name match: try to find in same file first, then globally
	// Skip init() — it's called implicitly by Go runtime, not by user code
	if call.Name == "init" && fr.Language == parser.LangGo {
		return "", "", resolutionUnresolved
	}
	if ids, ok := funcByName[call.Name]; ok {
		// Prefer same-file match
		var sameFileMatches []string
		for _, id := range ids {
			if node, ok := nodeMap[id]; ok {
				if node.FilePath == fr.FilePath && !node.IsInit {
					sameFileMatches = appendUnique(sameFileMatches, id)
				}
			}
		}
		if id, status := resolveUnique(sameFileMatches); id != "" {
			return id, EdgeConfidenceExact, resolutionResolved
		} else if status == resolutionAmbiguous {
			return "", "", resolutionAmbiguous
		}

		// Prefer same-package match (Go)
		if fr.Language == parser.LangGo && fr.Package != "" {
			var samePackageMatches []string
			for _, id := range ids {
				if node, ok := nodeMap[id]; ok {
					if node.Package == fr.Package && !node.IsInit {
						samePackageMatches = appendUnique(samePackageMatches, id)
					}
				}
			}
			if id, status := resolveUnique(samePackageMatches); id != "" {
				return id, EdgeConfidenceExact, resolutionResolved
			} else if status == resolutionAmbiguous {
				return "", "", resolutionAmbiguous
			}
		}
		// Fall back to a globally unique name match only.
		if len(ids) == 1 {
			return ids[0], EdgeConfidenceHeuristic, resolutionResolved
		}
		if len(ids) > 1 {
			return "", "", resolutionAmbiguous
		}
	}

	return "", "", resolutionUnresolved
}

// findContainingFunction finds the function node that contains the given line in a file.
func (b *Builder) findContainingFunction(fr *parser.FileResult, line int, fileID string, gd *GraphData) string {
	for _, fn := range fr.Functions {
		if line >= fn.StartLine && line <= fn.EndLine {
			return makeFuncID(fr.FilePath, fn.Name, fn.Receiver, fn.StartLine)
		}
	}
	return ""
}

// pathSegmentSuffix checks if filePath ends with the given suffix when split by path segments.
// e.g., pathSegmentSuffix("a/b/c/d", "c/d") → true, but pathSegmentSuffix("a/bc/d", "c/d") → false
func pathSegmentSuffix(filePath, suffix string) bool {
	if suffix == "" || filePath == "" {
		return false
	}
	fpParts := strings.Split(filepath.ToSlash(filePath), "/")
	sParts := strings.Split(filepath.ToSlash(suffix), "/")
	if len(sParts) > len(fpParts) {
		return false
	}
	for i := range sParts {
		if fpParts[len(fpParts)-len(sParts)+i] != sParts[i] {
			return false
		}
	}
	return true
}

// ID generation helpers

func makeFileID(path string) string {
	return fmt.Sprintf("file:%s", hashPath(path))
}

func makeFuncID(path, name, receiver string, line int) string {
	key := fmt.Sprintf("%s:%s:%s:%d", filepath.ToSlash(path), receiver, name, line)
	return fmt.Sprintf("func:%s", hashStr(key))
}

func makeClassID(path, name string) string {
	key := fmt.Sprintf("%s:%s", filepath.ToSlash(path), name)
	return fmt.Sprintf("class:%s", hashStr(key))
}

func hashPath(path string) string {
	h := sha256.Sum256([]byte(filepath.ToSlash(path)))
	return fmt.Sprintf("%x", h[:8])
}

func hashStr(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func fileBaseName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	// Handle .test.js, .spec.ts etc.
	for _, suffix := range []string{".test", ".spec"} {
		name = strings.TrimSuffix(name, suffix)
	}
	return name
}

func importBaseName(importPath string, lang parser.Language) string {
	switch lang {
	case parser.LangGo:
		parts := strings.Split(importPath, "/")
		return parts[len(parts)-1]
	case parser.LangJavaScript, parser.LangTypeScript:
		path := importPath
		for strings.HasPrefix(path, "../") {
			path = strings.TrimPrefix(path, "../")
		}
		path = strings.TrimPrefix(path, "./")
		parts := strings.Split(path, "/")
		name := parts[len(parts)-1]
		for _, ext := range []string{".ts", ".tsx", ".js", ".jsx"} {
			name = strings.TrimSuffix(name, ext)
		}
		return name
	case parser.LangPython:
		// Strip leading dots for relative imports
		clean := strings.TrimLeft(importPath, ".")
		if clean == "" {
			return ""
		}
		parts := strings.Split(clean, ".")
		return parts[len(parts)-1]
	}
	return ""
}

func fileQualifier(path string) string {
	normalized := canonicalPath(path)
	return strings.TrimSuffix(normalized, filepath.Ext(normalized))
}

func canonicalPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func appendUnique(ids []string, id string) []string {
	if id == "" {
		return ids
	}
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func uniqueID(ids []string) string {
	if len(ids) == 1 {
		return ids[0]
	}
	return ""
}

func resolveUnique(ids []string) (string, resolutionStatus) {
	switch len(ids) {
	case 0:
		return "", resolutionUnresolved
	case 1:
		return ids[0], resolutionResolved
	default:
		return "", resolutionAmbiguous
	}
}

func longestImportSuffixMatch(dirPath string, importParts []string) int {
	best := 0
	for i := range importParts {
		suffix := strings.Join(importParts[i:], "/")
		if suffix != "" && pathSegmentSuffix(dirPath, suffix) {
			score := len(importParts) - i
			if score > best {
				best = score
			}
		}
	}
	return best
}
