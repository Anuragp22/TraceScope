package parser

import (
	"context"
	"strings"

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
	parser.SetLanguage(p.lang)

	tree, err := parser.ParseCtx(context.Background(), nil, source)
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
		// Walk children to get the decorated function/class
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "function_definition" || child.Type() == "class_definition" {
				p.walk(child, source, result, currentClass)
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
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "dotted_name" {
				module = child.Content(source)
				break
			}
		}
		if module != "" {
			result.Imports = append(result.Imports, Import{
				Path: module,
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
		obj := findChildContent(funcNode, "identifier", source)
		attr := ""
		for i := 0; i < int(funcNode.ChildCount()); i++ {
			child := funcNode.Child(i)
			if child.Type() == "identifier" && child.Content(source) != obj {
				attr = child.Content(source)
			}
		}
		if attr != "" {
			result.Calls = append(result.Calls, FunctionCall{
				Name:     attr,
				Line:     line,
				Receiver: obj,
			})
		}
	}

	// Recurse into arguments for nested calls
	for i := 1; i < int(node.ChildCount()); i++ {
		p.walkForPyCalls(node.Child(i), source, result)
	}
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
	lower := strings.ToLower(path)
	return strings.Contains(lower, "test_") ||
		strings.Contains(lower, "_test.py") ||
		strings.Contains(lower, "tests/") ||
		strings.Contains(lower, "tests\\")
}
