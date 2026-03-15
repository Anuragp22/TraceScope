package parser

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

type PythonParser struct {
	lang *sitter.Language
}

func NewPythonParser() *PythonParser {
	return &PythonParser{lang: python.GetLanguage()}
}

func (p *PythonParser) Language() Language { return LangPython }

func (p *PythonParser) Parse(filePath string, source []byte) (*FileResult, error) {
	parser := sitter.NewParser()
	defer parser.Close()

	parser.SetLanguage(p.lang)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tree, err := parser.ParseCtx(ctx, nil, source)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	result := &FileResult{
		FilePath:   filePath,
		Language:   LangPython,
		IsTestFile: isPythonTestFile(filePath),
	}

	root := tree.RootNode()
	p.walk(root, source, result, "")

	return result, nil
}

func (p *PythonParser) walk(node *sitter.Node, source []byte, result *FileResult, currentClass string) {
	switch node.Type() {
	case "function_definition":
		name := findChildContent(node, "identifier", source)
		if name != "" {
			fd := FunctionDef{
				Name:      name,
				StartLine: int(node.StartPoint().Row) + 1,
				EndLine:   int(node.EndPoint().Row) + 1,
				IsExport:  !strings.HasPrefix(name, "_"),
			}
			if currentClass != "" {
				fd.Receiver = currentClass
			}
			result.Functions = append(result.Functions, fd)
		}
		// Recurse into function body for calls
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "block" {
				p.walk(child, source, result, currentClass)
			}
		}
		return

	case "class_definition":
		name := findChildContent(node, "identifier", source)
		if name != "" {
			result.Classes = append(result.Classes, ClassDef{
				Name:      name,
				StartLine: int(node.StartPoint().Row) + 1,
				EndLine:   int(node.EndPoint().Row) + 1,
				IsExport:  !strings.HasPrefix(name, "_"),
				Kind:      "class",
			})
			// Recurse with class context
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "block" {
					p.walk(child, source, result, name)
				}
			}
		}
		return

	case "decorated_definition":
		// Extract decorator names, then walk the decorated function/class
		var decorators []string
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "decorator" {
				// Decorator node: @name or @module.name
				decName := ""
				for j := 0; j < int(child.ChildCount()); j++ {
					gc := child.Child(j)
					if gc.Type() == "identifier" || gc.Type() == "attribute" || gc.Type() == "dotted_name" {
						decName = gc.Content(source)
					}
				}
				if decName != "" {
					decorators = append(decorators, decName)
				}
			} else if child.Type() == "function_definition" || child.Type() == "class_definition" {
				p.walk(child, source, result, currentClass)
				// Attach decorators to the last added function
				if child.Type() == "function_definition" && len(result.Functions) > 0 {
					result.Functions[len(result.Functions)-1].Decorators = decorators
				}
			}
		}
		return

	case "call":
		p.parseCallExpr(node, source, result)
		return

	case "import_statement":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "dotted_name" {
				result.Imports = append(result.Imports, Import{
					Path: child.Content(source),
					Line: int(node.StartPoint().Row) + 1,
				})
			}
		}

	case "import_from_statement":
		module := ""
		dots := ""
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			switch child.Type() {
			case "dotted_name":
				if module == "" {
					module = child.Content(source)
				}
			case "relative_import":
				// Handle relative imports: from .foo import bar, from ..foo import bar
				for j := 0; j < int(child.ChildCount()); j++ {
					gc := child.Child(j)
					switch gc.Type() {
					case "import_prefix":
						dots = strings.TrimRight(gc.Content(source), " ")
					case "dotted_name":
						module = gc.Content(source)
					}
				}
			}
		}
		path := dots + module
		if path != "" {
			result.Imports = append(result.Imports, Import{
				Path: path,
				Line: int(node.StartPoint().Row) + 1,
			})
		}
	}

	// Recurse
	for i := 0; i < int(node.ChildCount()); i++ {
		p.walk(node.Child(i), source, result, currentClass)
	}
}

func (p *PythonParser) parseCallExpr(node *sitter.Node, source []byte, result *FileResult) {
	if node.ChildCount() == 0 {
		return
	}
	funcNode := node.Child(0)
	line := int(node.StartPoint().Row) + 1

	switch funcNode.Type() {
	case "identifier":
		result.Calls = append(result.Calls, FunctionCall{
			Name: funcNode.Content(source),
			Line: line,
		})
	case "attribute":
		// Recursively flatten chained attribute access: a.b.c() → receiver="a.b", name="c"
		parts := flattenAttribute(funcNode, source)
		if len(parts) >= 2 {
			result.Calls = append(result.Calls, FunctionCall{
				Name:     parts[len(parts)-1],
				Line:     line,
				Receiver: strings.Join(parts[:len(parts)-1], "."),
			})
		} else if len(parts) == 1 {
			result.Calls = append(result.Calls, FunctionCall{
				Name: parts[0],
				Line: line,
			})
		}
	}

	// Recurse into arguments for nested calls
	for i := 1; i < int(node.ChildCount()); i++ {
		p.walkForPyCalls(node.Child(i), source, result)
	}
}

// flattenAttribute iteratively flattens nested attribute nodes into a list of names.
// e.g., a.b.c → ["a", "b", "c"]
// Uses iterative approach to avoid stack overflow on deeply nested chains.
func flattenAttribute(node *sitter.Node, source []byte) []string {
	// Collect attribute names from right to left, then reverse
	var parts []string
	cur := node
	for i := 0; i < 100; i++ { // depth limit to prevent infinite loops
		if cur.Type() == "identifier" {
			parts = append(parts, cur.Content(source))
			break
		}
		if cur.Type() == "attribute" && cur.ChildCount() >= 3 {
			attrNode := cur.Child(int(cur.ChildCount()) - 1)
			parts = append(parts, attrNode.Content(source))
			cur = cur.Child(0) // descend into object
			continue
		}
		parts = append(parts, cur.Content(source))
		break
	}
	// Reverse to get left-to-right order
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return parts
}

func (p *PythonParser) walkForPyCalls(node *sitter.Node, source []byte, result *FileResult) {
	if node.Type() == "call" {
		p.parseCallExpr(node, source, result)
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		p.walkForPyCalls(node.Child(i), source, result)
	}
}

func isPythonTestFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") {
		return true
	}
	// Check if file is in a tests/ or test/ directory
	dir := strings.ToLower(filepath.ToSlash(filepath.Dir(path)))
	return strings.HasSuffix(dir, "/tests") || strings.HasSuffix(dir, "/test") ||
		strings.Contains(dir, "/tests/") || strings.Contains(dir, "/test/")
}
