package parser

import (
	"context"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

type TypeScriptParser struct {
	tsLang  *sitter.Language
	tsxLang *sitter.Language
}

func NewTypeScriptParser() *TypeScriptParser {
	return &TypeScriptParser{
		tsLang:  typescript.GetLanguage(),
		tsxLang: tsx.GetLanguage(),
	}
}

func (p *TypeScriptParser) Language() Language { return LangTypeScript }

func (p *TypeScriptParser) Parse(filePath string, source []byte) (*FileResult, error) {
	parser := sitter.NewParser()
	defer parser.Close()

	// Use TSX grammar for .tsx files
	if strings.HasSuffix(filePath, ".tsx") {
		parser.SetLanguage(p.tsxLang)
	} else {
		parser.SetLanguage(p.tsLang)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tree, err := parser.ParseCtx(ctx, nil, source)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	result := &FileResult{
		FilePath:   filePath,
		Language:   LangTypeScript,
		IsTestFile: isJSTestFile(filePath),
	}

	root := tree.RootNode()
	p.walk(root, source, result)

	return result, nil
}

func (p *TypeScriptParser) walk(node *sitter.Node, source []byte, result *FileResult) {
	switch node.Type() {
	case "function_declaration":
		name := findChildContent(node, "identifier", source)
		if name != "" {
			result.Functions = append(result.Functions, FunctionDef{
				Name:      name,
				StartLine: int(node.StartPoint().Row) + 1,
				EndLine:   int(node.EndPoint().Row) + 1,
				IsExport:  isJSExported(node, source),
			})
		}

	case "lexical_declaration", "variable_declaration":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "variable_declarator" {
				name := findChildContent(child, "identifier", source)
				if name == "" {
					continue
				}
				for j := 0; j < int(child.ChildCount()); j++ {
					val := child.Child(j)
					if val.Type() == "arrow_function" || val.Type() == "function_expression" {
						result.Functions = append(result.Functions, FunctionDef{
							Name:      name,
							StartLine: int(node.StartPoint().Row) + 1,
							EndLine:   int(node.EndPoint().Row) + 1,
							IsExport:  isJSExported(node, source),
						})
						break
					}
				}
			}
		}

	case "class_declaration":
		name := ""
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "type_identifier" || child.Type() == "identifier" {
				name = child.Content(source)
				break
			}
		}
		if name != "" {
			result.Classes = append(result.Classes, ClassDef{
				Name:      name,
				StartLine: int(node.StartPoint().Row) + 1,
				EndLine:   int(node.EndPoint().Row) + 1,
				IsExport:  isJSExported(node, source),
				Kind:      "class",
			})
		}

	case "interface_declaration":
		name := ""
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "type_identifier" {
				name = child.Content(source)
				break
			}
		}
		if name != "" {
			result.Classes = append(result.Classes, ClassDef{
				Name:      name,
				StartLine: int(node.StartPoint().Row) + 1,
				EndLine:   int(node.EndPoint().Row) + 1,
				IsExport:  isJSExported(node, source),
				Kind:      "interface",
			})
		}

	case "type_alias_declaration":
		name := ""
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "type_identifier" {
				name = child.Content(source)
				break
			}
		}
		if name != "" {
			result.Classes = append(result.Classes, ClassDef{
				Name:      name,
				StartLine: int(node.StartPoint().Row) + 1,
				EndLine:   int(node.EndPoint().Row) + 1,
				IsExport:  isJSExported(node, source),
				Kind:      "type",
			})
		}

	case "method_definition":
		name := findChildContent(node, "property_identifier", source)
		if name != "" {
			result.Functions = append(result.Functions, FunctionDef{
				Name:      name,
				StartLine: int(node.StartPoint().Row) + 1,
				EndLine:   int(node.EndPoint().Row) + 1,
				IsExport:  true,
			})
		}

	case "call_expression":
		p.parseCallExpr(node, source, result)
		return

	case "import_statement":
		path := ""
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "string" {
				path = trimQuotes(child.Content(source))
			}
		}
		if path != "" {
			result.Imports = append(result.Imports, Import{
				Path: path,
				Line: int(node.StartPoint().Row) + 1,
			})
		}

	case "export_statement":
		// Walk children to find declarations within exports
		// Also handle: export default function() {} / export default function name() {}
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "function_expression" || child.Type() == "arrow_function" {
				name := findChildContent(child, "identifier", source)
				if name == "" {
					name = "default"
				}
				result.Functions = append(result.Functions, FunctionDef{
					Name:      name,
					StartLine: int(child.StartPoint().Row) + 1,
					EndLine:   int(child.EndPoint().Row) + 1,
					IsExport:  true,
				})
			} else {
				p.walk(child, source, result)
			}
		}
		return
	}

	// Recurse
	for i := 0; i < int(node.ChildCount()); i++ {
		p.walk(node.Child(i), source, result)
	}
}

func (p *TypeScriptParser) parseCallExpr(node *sitter.Node, source []byte, result *FileResult) {
	if node.ChildCount() == 0 {
		return
	}
	funcNode := node.Child(0)
	line := int(node.StartPoint().Row) + 1

	switch funcNode.Type() {
	case "identifier":
		name := funcNode.Content(source)
		if name == "require" {
			if node.ChildCount() > 1 {
				args := node.Child(1)
				if args.Type() == "arguments" && args.ChildCount() > 1 {
					arg := args.Child(1)
					if arg.Type() == "string" {
						result.Imports = append(result.Imports, Import{
							Path: trimQuotes(arg.Content(source)),
							Line: line,
						})
					}
				}
			}
		} else {
			result.Calls = append(result.Calls, FunctionCall{
				Name: name,
				Line: line,
			})
		}
	case "member_expression":
		obj := findChildContent(funcNode, "identifier", source)
		prop := findChildContent(funcNode, "property_identifier", source)
		if prop != "" {
			result.Calls = append(result.Calls, FunctionCall{
				Name:     prop,
				Line:     line,
				Receiver: obj,
			})
		}
	}

	for i := 1; i < int(node.ChildCount()); i++ {
		p.walkForTSCalls(node.Child(i), source, result)
	}
}

func (p *TypeScriptParser) walkForTSCalls(node *sitter.Node, source []byte, result *FileResult) {
	if node.Type() == "call_expression" {
		p.parseCallExpr(node, source, result)
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		p.walkForTSCalls(node.Child(i), source, result)
	}
}

