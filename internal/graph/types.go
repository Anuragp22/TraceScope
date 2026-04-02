package graph

// NodeType represents the type of a graph node.
type NodeType string

const (
	NodeFile     NodeType = "file"
	NodeFunction NodeType = "function"
	NodeClass    NodeType = "class"
)

// EdgeType represents the type of a graph edge.
type EdgeType string

const (
	EdgeContains   EdgeType = "CONTAINS"
	EdgeCalls      EdgeType = "CALLS"
	EdgeImports    EdgeType = "IMPORTS"
	EdgeExtends    EdgeType = "EXTENDS"
	EdgeImplements EdgeType = "IMPLEMENTS"
)

// EdgeConfidence describes how trustworthy a resolved edge is.
type EdgeConfidence string

const (
	EdgeConfidenceExact     EdgeConfidence = "EXACT"
	EdgeConfidenceHeuristic EdgeConfidence = "HEURISTIC"
)

// Node represents a node in the dependency graph.
type Node struct {
	ID        string   `json:"id"`
	Type      NodeType `json:"type"`
	Name      string   `json:"name"`
	FilePath  string   `json:"file_path"`
	StartLine int      `json:"start_line,omitempty"`
	EndLine   int      `json:"end_line,omitempty"`
	Package   string   `json:"package,omitempty"`
	Language  string   `json:"language"`
	IsExport  bool     `json:"is_export,omitempty"`
	IsTest    bool     `json:"is_test,omitempty"`
	IsInit    bool     `json:"is_init,omitempty"`
}

// Edge represents an edge in the dependency graph.
type Edge struct {
	Source     string         `json:"source"`
	Target     string         `json:"target"`
	Type       EdgeType       `json:"type"`
	Confidence EdgeConfidence `json:"confidence,omitempty"`
}

// ResolutionStats captures how many references resolved cleanly vs heuristically
// or were skipped because the target was ambiguous/unknown.
type ResolutionStats struct {
	ExactCallEdges        int `json:"exact_call_edges,omitempty"`
	HeuristicCallEdges    int `json:"heuristic_call_edges,omitempty"`
	AmbiguousCalls        int `json:"ambiguous_calls,omitempty"`
	UnresolvedCalls       int `json:"unresolved_calls,omitempty"`
	ExactInheritance      int `json:"exact_inheritance_edges,omitempty"`
	HeuristicInheritance  int `json:"heuristic_inheritance_edges,omitempty"`
	AmbiguousInheritance  int `json:"ambiguous_inheritance,omitempty"`
	UnresolvedInheritance int `json:"unresolved_inheritance,omitempty"`
}

// FileMetadata stores per-file indexing metadata for incremental indexing.
type FileMetadata struct {
	Hash      string `json:"hash"`
	Language  string `json:"language"`
	ParsedAt  int64  `json:"parsed_at"`
	NodeCount int    `json:"node_count"`
}

// GraphData is the serializable representation of the full dependency graph.
type GraphData struct {
	Nodes           []Node                   `json:"nodes"`
	Edges           []Edge                   `json:"edges"`
	RootPath        string                   `json:"root_path,omitempty"`
	FileMetadata    map[string]*FileMetadata `json:"file_metadata,omitempty"`
	ResolutionStats ResolutionStats          `json:"resolution_stats,omitempty"`
}

// Metadata holds graph statistics.
type Metadata struct {
	TotalFiles     int            `json:"total_files"`
	TotalFunctions int            `json:"total_functions"`
	TotalClasses   int            `json:"total_classes"`
	TotalEdges     int            `json:"total_edges"`
	Languages      map[string]int `json:"languages"`
}
