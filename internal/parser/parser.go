package parser

// FunctionDef represents a function or method definition.
type FunctionDef struct {
	Name      string
	StartLine int
	EndLine   int
	Receiver  string // Go method receiver type (empty for functions)
	IsExport  bool   // exported/public
}

// FunctionCall represents a function call expression.
type FunctionCall struct {
	Name     string
	Line     int
	Receiver string // e.g., "pkg.Func" → receiver is "pkg"
}

// Import represents an import statement.
type Import struct {
	Path  string
	Alias string
	Line  int
}

// ClassDef represents a class, struct, or interface definition.
type ClassDef struct {
	Name      string
	StartLine int
	EndLine   int
	IsExport  bool
	Kind      string // "class", "struct", "interface"
}

// FileResult holds all parsed data from a single file.
type FileResult struct {
	FilePath   string
	Language   Language
	Package    string // Go package name
	Functions  []FunctionDef
	Calls      []FunctionCall
	Imports    []Import
	Classes    []ClassDef
	IsTestFile bool
}

// LanguageParser is the interface all language parsers must implement.
type LanguageParser interface {
	Parse(filePath string, source []byte) (*FileResult, error)
	Language() Language
}
