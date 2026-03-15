package parser

import (
	"context"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

type JavaScriptParser struct {
	lang *sitter.Language
}

func NewJavaScriptParser() *JavaScriptParser {
	return &JavaScriptParser{lang: javascript.GetLanguage()}
}

func (p *JavaScriptParser) Language() Language { return LangJavaScript }

func (p *JavaScriptParser) Parse(filePath string, source []byte) (*FileResult, error) {
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
		Language:   LangJavaScript,
		IsTestFile: isJSTestFile(filePath),
	}

	root := tree.RootNode()
	p.walk(root, source, result)

	return result, nil
}

func (p *JavaScriptParser) walk(node *sitter.Node, source []byte, result *FileResult) {
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
				// Check if value is arrow_function or function_expression
				for j := 0; j < int(child.ChildCount()); j++ {
					val := child.Child(j)
					if val.Type() == "arrow_function" || val.Type() == "function_expression" || val.Type() == "function" {
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
		name := findChildContent(node, "identifier", source)
		if name != "" {
			var bases []string
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "class_heritage" {
					for j := 0; j < int(child.ChildCount()); j++ {
						gc := child.Child(j)
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
			// Recurse with class context for methods
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

	case "method_definition":
		// Standalone method (not inside a class we're tracking)
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
		return // handle recursion ourselves

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
			// Handle anonymous default export functions
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

func (p *JavaScriptParser) parseCallExpr(node *sitter.Node, source []byte, result *FileResult) {
	if node.ChildCount() == 0 {
		return
	}
	funcNode := node.Child(0)
	line := int(node.StartPoint().Row) + 1

	switch funcNode.Type() {
	case "identifier":
		name := funcNode.Content(source)
		// Check for require()
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

	// Recurse into arguments for nested calls
	for i := 1; i < int(node.ChildCount()); i++ {
		p.walkForCalls(node.Child(i), source, result)
	}
}

func (p *JavaScriptParser) walkForCalls(node *sitter.Node, source []byte, result *FileResult) {
	if node.Type() == "call_expression" {
		p.parseCallExpr(node, source, result)
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		p.walkForCalls(node.Child(i), source, result)
	}
}

func findChildContent(node *sitter.Node, childType string, source []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == childType {
			return child.Content(source)
		}
	}
	return ""
}

func trimQuotes(s string) string {
	s = strings.TrimPrefix(s, "\"")
	s = strings.TrimSuffix(s, "\"")
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimSuffix(s, "'")
	s = strings.TrimPrefix(s, "`")
	s = strings.TrimSuffix(s, "`")
	return s
}

func isJSExported(node *sitter.Node) bool {
	parent := node.Parent()
	if parent != nil && parent.Type() == "export_statement" {
		return true
	}
	return false
}

func isJSTestFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, ".test.") ||
		strings.Contains(lower, ".spec.") ||
		strings.Contains(lower, "__tests__") ||
		strings.Contains(lower, "_test.")
}
