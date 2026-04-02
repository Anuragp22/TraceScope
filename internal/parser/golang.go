package parser

import (
	"go/ast"
	"go/importer"
	goparser "go/parser"
	"go/token"
	"go/types"
	"strings"
)

// GoParser parses Go source files using the standard library's go/parser and go/ast.
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
				IsInit:    name == "init" && receiver == "",
			})
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					kind := ""
					var bases []string
					switch iface := ts.Type.(type) {
					case *ast.StructType:
						kind = "struct"
						// Extract embedded (anonymous) fields as bases
						if iface.Fields != nil {
							for _, field := range iface.Fields.List {
								if len(field.Names) == 0 {
									if name := typeExprName(field.Type); name != "" {
										bases = append(bases, name)
									}
								}
							}
						}
					case *ast.InterfaceType:
						kind = "interface"
						if iface.Methods != nil {
							for _, method := range iface.Methods.List {
								if len(method.Names) > 0 {
									// Named method signature
									if _, ok := method.Type.(*ast.FuncType); ok {
										mName := method.Names[0].Name
										result.Functions = append(result.Functions, FunctionDef{
											Name:      mName,
											StartLine: fset.Position(method.Pos()).Line,
											EndLine:   fset.Position(method.End()).Line,
											Receiver:  ts.Name.Name,
											IsExport:  ast.IsExported(mName),
										})
									}
								} else {
									// Embedded interface (anonymous type reference)
									if name := typeExprName(method.Type); name != "" {
										bases = append(bases, name)
									}
								}
							}
						}
					}
					if kind != "" {
						result.Classes = append(result.Classes, ClassDef{
							Name:      ts.Name.Name,
							StartLine: fset.Position(ts.Pos()).Line,
							EndLine:   fset.Position(ts.End()).Line,
							IsExport:  ast.IsExported(ts.Name.Name),
							Kind:      kind,
							Bases:     bases,
						})
					}
				}
			}
		}
	}

	// Walk entire AST for call expressions — handles closures, nested calls, etc.
	selections := inferGoSelections(fset, file)

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
			parsedCall := FunctionCall{
				Name:     fn.Sel.Name,
				Line:     line,
				Receiver: receiver,
			}
			if selection := selections[fn]; selection != nil {
				if selection.Kind() == types.MethodVal || selection.Kind() == types.MethodExpr {
					parsedCall.ReceiverType, parsedCall.ReceiverPackage = goReceiverTypeInfo(selection.Recv())
					if parsedCall.ReceiverPackage == "" {
						parsedCall.ReceiverPackage = result.Package
					}
				}
			}
			result.Calls = append(result.Calls, parsedCall)
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
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

// typeExprName extracts the simple type name from a type expression.
// Handles *Foo, pkg.Foo, *pkg.Foo, etc.
func typeExprName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return typeExprName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.IndexExpr:
		return typeExprName(t.X)
	case *ast.IndexListExpr:
		return typeExprName(t.X)
	}
	return ""
}

func inferGoSelections(fset *token.FileSet, file *ast.File) map[*ast.SelectorExpr]*types.Selection {
	info := &types.Info{
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	cfg := &types.Config{
		Importer: importer.Default(),
		Error:    func(error) {},
	}
	_, _ = cfg.Check(file.Name.Name, fset, []*ast.File{file}, info)
	return info.Selections
}

func goReceiverTypeInfo(recv types.Type) (string, string) {
	for {
		switch t := recv.(type) {
		case *types.Pointer:
			recv = t.Elem()
		case *types.Named:
			pkgName := ""
			if obj := t.Obj(); obj != nil {
				if pkg := obj.Pkg(); pkg != nil {
					pkgName = pkg.Name()
				}
				return obj.Name(), pkgName
			}
			return "", ""
		default:
			return "", ""
		}
	}
}
