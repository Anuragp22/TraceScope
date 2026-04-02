package parser

// FunctionDef represents a function or method definition.
type FunctionDef struct {
	Name       string
	StartLine  int
	EndLine    int
	Receiver   string   // Go method receiver type (empty for functions)
	IsExport   bool     // exported/public
	IsInit     bool     // Go init() function
	Decorators []string // Python decorators (@property, @staticmethod, etc.)
}

// FunctionCall represents a function call expression.
type FunctionCall struct {
	Name            string
	Line            int
	Receiver        string // lexical selector receiver, e.g. "pkg" or "user"
	ReceiverType    string // static receiver type for method calls, e.g. "User"
	ReceiverPackage string // static receiver package for method calls, e.g. "models"
}

// Import represents an import statement.
type Import struct {
	Path   string
	Alias  string // local binding name used in this file
	Symbol string // exported name in the source module: "default", "*", or a named symbol
	Line   int
}

// ClassDef represents a class, struct, or interface definition.
type ClassDef struct {
	Name      string
	StartLine int
	EndLine   int
	IsExport  bool
	Kind      string   // "class", "struct", "interface", "type"
	Bases     []string // parent classes, embedded structs, extended interfaces
}

// FileResult holds all parsed data from a single file.
type FileResult struct {
	FilePath    string
	Language    Language
	Package     string // Go package name
	Functions   []FunctionDef
	Calls       []FunctionCall
	Imports     []Import
	Classes     []ClassDef
	IsTestFile  bool
	ContentHash string // SHA-256 hex of source bytes
}

// LanguageParser is the interface all language parsers must implement.
type LanguageParser interface {
	Parse(filePath string, source []byte) (*FileResult, error)
	Language() Language
}
