package graph

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

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
	funcByQualified := make(map[string]string)
	fileNodeByPath := make(map[string]string)
	importMap := make(map[string]map[string]string)
	fileByDir := make(map[string][]*parser.FileResult)

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
			}
			nodeMap[funcID] = funcNode

			// CONTAINS edge: file → function
			gd.Edges = append(gd.Edges, Edge{
				Source: fileID,
				Target: funcID,
				Type:   EdgeContains,
			})

			// Register in lookup maps
			funcByName[fn.Name] = append(funcByName[fn.Name], funcID)

			// Qualified names for Go
			if fr.Language == parser.LangGo && fr.Package != "" {
				if fn.Receiver != "" {
					key := fr.Package + "." + fn.Receiver + "." + fn.Name
					funcByQualified[key] = funcID
				}
				key := fr.Package + "." + fn.Name
				funcByQualified[key] = funcID
			}

			// For JS/TS/Python: use file base name as pseudo-package
			if fr.Language != parser.LangGo {
				baseName := fileBaseName(fr.FilePath)
				key := baseName + "." + fn.Name
				funcByQualified[key] = funcID
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

			// CONTAINS edge: file → class
			gd.Edges = append(gd.Edges, Edge{
				Source: fileID,
				Target: classID,
				Type:   EdgeContains,
			})
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
					Source: fileID,
					Target: targetFileID,
					Type:   EdgeImports,
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
			targetID := b.resolveCall(fr, call, funcByName, funcByQualified, importMap, fileNodeByPath, nodeMap)
			if targetID == "" {
				continue
			}

			// Find the calling function (the function that contains this call line)
			callerID := b.findContainingFunction(fr, call.Line, fileNodeByPath[fr.FilePath], gd)
			if callerID == "" {
				// If we can't find the containing function, use the file
				callerID = fileNodeByPath[fr.FilePath]
			}

			gd.Edges = append(gd.Edges, Edge{
				Source: callerID,
				Target: targetID,
				Type:   EdgeCalls,
			})
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
	// For Go, imports are package paths. Match by path-segment suffix.
	// Prefer the longest (most specific) path match for determinism.
	bestID := ""
	bestLen := 0
	for _, r := range results {
		if r.Language != parser.LangGo {
			continue
		}
		filePath := filepath.ToSlash(r.FilePath)
		if pathSegmentSuffix(filePath, imp.Path) {
			if id, ok := fileNodeByPath[r.FilePath]; ok {
				if len(filePath) > bestLen {
					bestID = id
					bestLen = len(filePath)
				}
			}
		}
	}
	return bestID
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
		candidate := resolved + ext
		if id, ok := fileNodeByPath[candidate]; ok {
			return id
		}
	}

	// Try index files
	indexFiles := []string{"index.ts", "index.tsx", "index.js", "index.jsx"}
	for _, idx := range indexFiles {
		candidate := filepath.Join(resolved, idx)
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
		candidate := filepath.Join(dir, "__init__.py")
		if id, ok := fileNodeByPath[candidate]; ok {
			return id
		}
		return ""
	}

	relPath := filepath.Join(append([]string{dir}, parts...)...)
	candidates := []string{
		relPath + ".py",
		filepath.Join(relPath, "__init__.py"),
	}
	for _, c := range candidates {
		if id, ok := fileNodeByPath[c]; ok {
			return id
		}
	}

	// For absolute imports, use path-segment matching
	if dots == 0 && len(parts) > 0 {
		// Try matching by segments from right
		for knownPath, id := range fileNodeByPath {
			if !strings.HasSuffix(knownPath, ".py") {
				continue
			}
			if pythonPathMatch(knownPath, parts) {
				return id
			}
		}
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
func (b *Builder) resolveCall(fr *parser.FileResult, call parser.FunctionCall, funcByName map[string][]string, funcByQualified map[string]string, importMap map[string]map[string]string, fileNodeByPath map[string]string, nodeMap map[string]*Node) string {
	// If call has a receiver (e.g., pkg.Func, obj.Method)
	if call.Receiver != "" {
		// Try qualified lookup: receiver.name
		key := call.Receiver + "." + call.Name
		if id, ok := funcByQualified[key]; ok {
			return id
		}

		// For Go: try package.name via import map
		if fr.Language == parser.LangGo {
			if fileImports, ok := importMap[fr.FilePath]; ok {
				if targetFileID, ok := fileImports[call.Receiver]; ok {
					// Find the function in the target file's package
					if targetNode, ok := nodeMap[targetFileID]; ok {
						pkg := targetNode.Package
						qualKey := pkg + "." + call.Name
						if id, ok := funcByQualified[qualKey]; ok {
							return id
						}
					}
				}
			}
		}

		// For JS/TS: try import alias resolution
		if fr.Language == parser.LangJavaScript || fr.Language == parser.LangTypeScript {
			if fileImports, ok := importMap[fr.FilePath]; ok {
				if targetFileID, ok := fileImports[call.Receiver]; ok {
					// Find function in the imported file
					baseName := ""
					if targetNode, ok := nodeMap[targetFileID]; ok {
						baseName = fileBaseName(targetNode.FilePath)
					}
					if baseName != "" {
						qualKey := baseName + "." + call.Name
						if id, ok := funcByQualified[qualKey]; ok {
							return id
						}
					}
				}
			}
		}
	}

	// Simple name match: try to find in same file first, then globally
	if ids, ok := funcByName[call.Name]; ok {
		// Prefer same-file match
		for _, id := range ids {
			if node, ok := nodeMap[id]; ok {
				if node.FilePath == fr.FilePath {
					return id
				}
			}
		}
		// Prefer same-package match (Go)
		if fr.Language == parser.LangGo && fr.Package != "" {
			for _, id := range ids {
				if node, ok := nodeMap[id]; ok {
					if node.Package == fr.Package {
						return id
					}
				}
			}
		}
		// Fall back to first match
		if len(ids) == 1 {
			return ids[0]
		}
	}

	return ""
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
