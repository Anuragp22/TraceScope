package parser

import (
	"strings"

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
	lang := p.tsLang
	if strings.HasSuffix(filePath, ".tsx") {
		lang = p.tsxLang
	}

	tree, err := parseWithTreeSitter(lang, source)
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
				IsExport:  isJSExported(node),
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
							IsExport:  isJSExported(node),
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
			var bases []string
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "class_heritage" {
					for j := 0; j < int(child.ChildCount()); j++ {
						gc := child.Child(j)
						if gc.Type() == "extends_clause" || gc.Type() == "implements_clause" {
							for k := 0; k < int(gc.ChildCount()); k++ {
								tc := gc.Child(k)
								if tc.Type() == "type_identifier" || tc.Type() == "identifier" {
									bases = append(bases, tc.Content(source))
								}
							}
						}
						// Direct identifier in heritage (JS-style)
						if gc.Type() == "identifier" || gc.Type() == "type_identifier" {
							bases = append(bases, gc.Content(source))
						}
					}
				}
			}
			result.Classes = append(result.Classes, ClassDef{
				Name:      name,
				StartLine: int(node.StartPoint().Row) + 1,
				EndLine:   int(node.EndPoint().Row) + 1,
				IsExport:  isJSExported(node),
				Kind:      "class",
				Bases:     bases,
			})
			// Recurse into class body for methods with class context
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "class_body" {
					for j := 0; j < int(child.ChildCount()); j++ {
						gc := child.Child(j)
						if gc.Type() == "method_definition" {
							mName := findChildContent(gc, "property_identifier", source)
							if mName != "" {
								result.Functions = append(result.Functions, FunctionDef{
									Name:      mName,
									StartLine: int(gc.StartPoint().Row) + 1,
									EndLine:   int(gc.EndPoint().Row) + 1,
									IsExport:  true,
									Receiver:  name,
								})
							}
						} else {
							p.walk(gc, source, result)
						}
					}
				}
			}
		}
		return

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
				IsExport:  isJSExported(node),
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
				IsExport:  isJSExported(node),
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
		result.Imports = append(result.Imports, parseJSImportStatement(node, source)...)

	case "export_statement":
		// Walk children to find declarations within exports
		// Also handle: export default function() {} / export default function name() {}
		isDefaultExport := hasJSChildType(node, "default")
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
			} else if isDefaultExport && child.Type() == "function_declaration" {
				result.Functions = append(result.Functions, FunctionDef{
					Name:      "default",
					StartLine: int(child.StartPoint().Row) + 1,
					EndLine:   int(child.EndPoint().Row) + 1,
					IsExport:  true,
				})
				p.walk(child, source, result)
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
						alias := ""
						if parent := node.Parent(); parent != nil && parent.Type() == "variable_declarator" {
							alias = findChildContent(parent, "identifier", source)
						}
						result.Imports = append(result.Imports, Import{
							Path:   trimQuotes(arg.Content(source)),
							Alias:  alias,
							Symbol: "*",
							Line:   line,
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
