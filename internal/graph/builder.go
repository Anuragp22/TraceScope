package graph

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
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
	// funcByName maps "name" → list of function node IDs (for simple name matching)
	funcByName := make(map[string][]string)
	// funcByQualified maps "package.name" or "receiver.name" → node ID
	funcByQualified := make(map[string]string)
	// fileNodeByPath maps file path → file node ID
	fileNodeByPath := make(map[string]string)
	// importMap maps (file path, alias/basename) → imported file path
	importMap := make(map[string]map[string]string)
	// fileByDir maps directory → list of file results (for Go package resolution)
	fileByDir := make(map[string][]*parser.FileResult)
	// nodeMap for quick lookup
	nodeMap := make(map[string]*Node)

	// ── Pass 1: Register definitions ──

	for _, fr := range results {
		// File node
		fileID := makeFileID(fr.FilePath)
		fileNode := Node{
			ID:       fileID,
			Type:     NodeFile,
			Name:     filepath.Base(fr.FilePath),
			FilePath: fr.FilePath,
			Language: string(fr.Language),
			Package:  fr.Package,
			IsTest:   fr.IsTestFile,
		}
		gd.Nodes = append(gd.Nodes, fileNode)
		fileNodeByPath[fr.FilePath] = fileID
		nodeMap[fileID] = &gd.Nodes[len(gd.Nodes)-1]

		dir := filepath.Dir(fr.FilePath)
		fileByDir[dir] = append(fileByDir[dir], fr)

		// Function nodes
		for _, fn := range fr.Functions {
			funcID := makeFuncID(fr.FilePath, fn.Name, fn.Receiver, fn.StartLine)
			funcNode := Node{
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
			gd.Nodes = append(gd.Nodes, funcNode)
			nodeMap[funcID] = &gd.Nodes[len(gd.Nodes)-1]

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
			classNode := Node{
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
			gd.Nodes = append(gd.Nodes, classNode)
			nodeMap[classID] = &gd.Nodes[len(gd.Nodes)-1]

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
	// For Go, imports are package paths. Find any file whose package matches.
	// We match by checking if the file path contains the import path suffix.
	for _, r := range results {
		if r.Language != parser.LangGo {
			continue
		}
		filePath := filepath.ToSlash(r.FilePath)
		if strings.Contains(filePath, imp.Path) {
			if id, ok := fileNodeByPath[r.FilePath]; ok {
				return id
			}
		}
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
	// Convert dotted path to file path
	parts := strings.Split(imp.Path, ".")
	dir := filepath.Dir(currentFile)

	// Try relative import
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

	// Try from project root - search all files
	fileName := parts[len(parts)-1] + ".py"
	for path, id := range fileNodeByPath {
		if strings.HasSuffix(path, fileName) {
			return id
		}
	}

	return ""
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

// ID generation helpers

func makeFileID(path string) string {
	return fmt.Sprintf("file:%s", hashPath(path))
}

func makeFuncID(path, name, receiver string, line int) string {
	key := fmt.Sprintf("%s:%s:%s:%d", path, receiver, name, line)
	return fmt.Sprintf("func:%s", hashStr(key))
}

func makeClassID(path, name string) string {
	key := fmt.Sprintf("%s:%s", path, name)
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
		path := strings.TrimPrefix(importPath, "./")
		path = strings.TrimPrefix(path, "../")
		parts := strings.Split(path, "/")
		name := parts[len(parts)-1]
		for _, ext := range []string{".ts", ".tsx", ".js", ".jsx"} {
			name = strings.TrimSuffix(name, ext)
		}
		return name
	case parser.LangPython:
		parts := strings.Split(importPath, ".")
		return parts[len(parts)-1]
	}
	return ""
}
