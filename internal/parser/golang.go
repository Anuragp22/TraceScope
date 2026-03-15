package parser

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// GoParser parses Go source files using the standard library's go/parser and go/ast.
// This replaces tree-sitter for Go, providing compiler-accurate AST parsing with
// proper receiver resolution, unicode-safe export detection, and closure handling.
type GoParser struct{}

func NewGoParser() *GoParser {
	return &GoParser{}
}

func (p *GoParser) Language() Language { return LangGo }

func (p *GoParser) Parse(filePath string, source []byte) (*FileResult, error) {
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, filePath, source, goparser.AllErrors)
	if err != nil && file == nil {
		return nil, err
	}

	result := &FileResult{
		FilePath:   filePath,
		Language:   LangGo,
		Package:    file.Name.Name,
		IsTestFile: strings.HasSuffix(filePath, "_test.go"),
	}

	// Extract imports from the AST
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		result.Imports = append(result.Imports, Import{
			Path:  importPath,
			Alias: alias,
			Line:  fset.Position(imp.Pos()).Line,
		})
	}

	// Extract declarations (functions, methods, types)
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			name := d.Name.Name
			receiver := ""
			if d.Recv != nil && d.Recv.NumFields() > 0 {
				receiver = receiverTypeName(d.Recv.List[0].Type)
			}
			result.Functions = append(result.Functions, FunctionDef{
				Name:      name,
				StartLine: fset.Position(d.Pos()).Line,
				EndLine:   fset.Position(d.End()).Line,
				Receiver:  receiver,
				IsExport:  ast.IsExported(name),
			})
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					kind := ""
					switch ts.Type.(type) {
					case *ast.StructType:
						kind = "struct"
					case *ast.InterfaceType:
						kind = "interface"
					}
					if kind != "" {
						result.Classes = append(result.Classes, ClassDef{
							Name:      ts.Name.Name,
							StartLine: fset.Position(ts.Pos()).Line,
							EndLine:   fset.Position(ts.End()).Line,
							IsExport:  ast.IsExported(ts.Name.Name),
							Kind:      kind,
						})
					}
				}
			}
		}
	}

	// Walk entire AST for call expressions — handles closures, nested calls, etc.
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		line := fset.Position(call.Pos()).Line
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			result.Calls = append(result.Calls, FunctionCall{
				Name: fn.Name,
				Line: line,
			})
		case *ast.SelectorExpr:
			receiver := ""
			if ident, ok := fn.X.(*ast.Ident); ok {
				receiver = ident.Name
			}
			result.Calls = append(result.Calls, FunctionCall{
				Name:     fn.Sel.Name,
				Line:     line,
				Receiver: receiver,
			})
		}
		return true
	})

	return result, nil
}

// receiverTypeName extracts the type name from a method receiver expression,
// unwrapping pointer types and generic type parameters.
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.IndexListExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// GoQualifiedName returns the fully qualified name for a Go function.
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
