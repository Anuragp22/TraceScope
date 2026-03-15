package parser

import (
	"context"
	"path/filepath"
	"strings"
	"unicode"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

type GoParser struct {
	lang *sitter.Language
}

func NewGoParser() *GoParser {
	return &GoParser{lang: golang.GetLanguage()}
}

func (p *GoParser) Language() Language { return LangGo }

func (p *GoParser) Parse(filePath string, source []byte) (*FileResult, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(p.lang)

	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	result := &FileResult{
		FilePath:   filePath,
		Language:   LangGo,
		IsTestFile: strings.HasSuffix(filePath, "_test.go"),
	}

	root := tree.RootNode()
	p.walk(root, source, result)

	return result, nil
}

func (p *GoParser) walk(node *sitter.Node, source []byte, result *FileResult) {
	switch node.Type() {
	case "package_clause":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "package_identifier" {
				result.Package = child.Content(source)
			}
		}

	case "function_declaration":
		name := ""
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "identifier" {
				name = child.Content(source)
				break
			}
		}
		if name != "" {
			result.Functions = append(result.Functions, FunctionDef{
				Name:      name,
				StartLine: int(node.StartPoint().Row) + 1,
				EndLine:   int(node.EndPoint().Row) + 1,
				IsExport:  isGoExported(name),
			})
		}

	case "method_declaration":
		name := ""
		receiver := ""
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			switch child.Type() {
			case "field_identifier":
				name = child.Content(source)
			case "parameter_list":
				// Extract receiver type
				for j := 0; j < int(child.ChildCount()); j++ {
					param := child.Child(j)
					if param.Type() == "parameter_declaration" {
						for k := 0; k < int(param.ChildCount()); k++ {
							typeNode := param.Child(k)
							t := typeNode.Type()
							if t == "type_identifier" || t == "pointer_type" {
								receiver = extractTypeName(typeNode, source)
							}
						}
					}
				}
			}
		}
		if name != "" {
			result.Functions = append(result.Functions, FunctionDef{
				Name:      name,
				StartLine: int(node.StartPoint().Row) + 1,
				EndLine:   int(node.EndPoint().Row) + 1,
				Receiver:  receiver,
				IsExport:  isGoExported(name),
			})
		}

	case "call_expression":
		p.parseCallExpr(node, source, result)
		// Don't recurse into children for calls, we handle it here
		return

	case "import_spec":
		alias := ""
		path := ""
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			switch child.Type() {
			case "package_identifier":
				alias = child.Content(source)
			case "interpreted_string_literal":
				path = strings.Trim(child.Content(source), "\"")
			}
		}
		if path != "" {
			result.Imports = append(result.Imports, Import{
				Path:  path,
				Alias: alias,
				Line:  int(node.StartPoint().Row) + 1,
			})
		}

	case "type_declaration":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "type_spec" {
				p.parseTypeSpec(child, source, result)
			}
		}
	}

	// Recurse
	for i := 0; i < int(node.ChildCount()); i++ {
		p.walk(node.Child(i), source, result)
	}
}

func (p *GoParser) parseCallExpr(node *sitter.Node, source []byte, result *FileResult) {
	if node.ChildCount() == 0 {
		return
	}
	funcNode := node.Child(0)
	if funcNode == nil {
		return
	}

	line := int(node.StartPoint().Row) + 1

	switch funcNode.Type() {
	case "identifier":
		result.Calls = append(result.Calls, FunctionCall{
			Name: funcNode.Content(source),
			Line: line,
		})
	case "selector_expression":
		operand := ""
		selector := ""
		for i := 0; i < int(funcNode.ChildCount()); i++ {
			child := funcNode.Child(i)
			switch child.Type() {
			case "identifier":
				operand = child.Content(source)
			case "field_identifier":
				selector = child.Content(source)
			}
		}
		if selector != "" {
			result.Calls = append(result.Calls, FunctionCall{
				Name:     selector,
				Line:     line,
				Receiver: operand,
			})
		}
	}

	// Recurse into arguments for nested calls
	for i := 1; i < int(node.ChildCount()); i++ {
		p.walkForCalls(node.Child(i), source, result)
	}
}

func (p *GoParser) walkForCalls(node *sitter.Node, source []byte, result *FileResult) {
	if node.Type() == "call_expression" {
		p.parseCallExpr(node, source, result)
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		p.walkForCalls(node.Child(i), source, result)
	}
}

func (p *GoParser) parseTypeSpec(node *sitter.Node, source []byte, result *FileResult) {
	name := ""
	kind := ""
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "type_identifier":
			name = child.Content(source)
		case "struct_type":
			kind = "struct"
		case "interface_type":
			kind = "interface"
		}
	}
	if name != "" && kind != "" {
		result.Classes = append(result.Classes, ClassDef{
			Name:      name,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			IsExport:  isGoExported(name),
			Kind:      kind,
		})
	}
}

func extractTypeName(node *sitter.Node, source []byte) string {
	content := node.Content(source)
	content = strings.TrimPrefix(content, "*")
	return content
}

func isGoExported(name string) bool {
	if len(name) == 0 {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

// QualifiedName returns the fully qualified name for a Go function.
func GoQualifiedName(pkg, receiver, name string) string {
	parts := []string{}
	if pkg != "" {
		parts = append(parts, pkg)
	}
	if receiver != "" {
		parts = append(parts, receiver)
	}
	parts = append(parts, name)
	return strings.Join(parts, ".")
}

// GoImportBaseName returns the base package name from an import path.
func GoImportBaseName(importPath string) string {
	parts := strings.Split(importPath, "/")
	return parts[len(parts)-1]
}

// RelativeGoImportPath converts an absolute file path to a Go import-style relative path.
func RelativeGoImportPath(basePath, filePath string) string {
	rel, err := filepath.Rel(basePath, filepath.Dir(filePath))
	if err != nil {
		return filePath
	}
	return filepath.ToSlash(rel)
}
