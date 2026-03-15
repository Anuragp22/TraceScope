package analyzer

import (
	"path/filepath"
	"strings"

	"github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
)

// ChangedFunction represents a function that was modified in the diff.
type ChangedFunction struct {
	NodeID   string
	Node     *graph.Node
	FilePath string
}

// MapDiffToFunctions finds which function nodes overlap with changed line ranges.
func MapDiffToFunctions(changedFiles []diff.ChangedFile, graphData *graph.GraphData) []ChangedFunction {
	// Build lookup: normalized file path → list of function nodes
	funcsByFile := make(map[string][]*graph.Node)
	for i := range graphData.Nodes {
		n := &graphData.Nodes[i]
		if n.Type == graph.NodeFunction {
			normPath := normalizePath(n.FilePath)
			funcsByFile[normPath] = append(funcsByFile[normPath], n)
		}
	}

	var result []ChangedFunction

	for _, cf := range changedFiles {
		normDiffPath := normalizePath(cf.Path)

		// Try to match the diff path to a known file
		var matchedPath string
		for knownPath := range funcsByFile {
			if strings.HasSuffix(knownPath, normDiffPath) || strings.HasSuffix(normDiffPath, knownPath) || knownPath == normDiffPath {
				matchedPath = knownPath
				break
			}
		}

		if matchedPath == "" {
			continue
		}

		funcs := funcsByFile[matchedPath]
		for _, fn := range funcs {
			if cf.IsNew {
				// New file — all functions are changed
				result = append(result, ChangedFunction{
					NodeID:   fn.ID,
					Node:     fn,
					FilePath: fn.FilePath,
				})
				continue
			}

			// Check if any changed line range overlaps with the function's line range
			for _, lr := range cf.LineRanges {
				if linesOverlap(lr.Start, lr.End, fn.StartLine, fn.EndLine) {
					result = append(result, ChangedFunction{
						NodeID:   fn.ID,
						Node:     fn,
						FilePath: fn.FilePath,
					})
					break
				}
			}
		}
	}

	return result
}

// MapDiffToFileNodes finds which file nodes match changed files.
func MapDiffToFileNodes(changedFiles []diff.ChangedFile, graphData *graph.GraphData) []string {
	fileNodes := make(map[string]string) // normalized path → node ID
	for _, n := range graphData.Nodes {
		if n.Type == graph.NodeFile {
			fileNodes[normalizePath(n.FilePath)] = n.ID
		}
	}

	var result []string
	for _, cf := range changedFiles {
		normPath := normalizePath(cf.Path)
		for knownPath, id := range fileNodes {
			if strings.HasSuffix(knownPath, normPath) || strings.HasSuffix(normPath, knownPath) || knownPath == normPath {
				result = append(result, id)
				break
			}
		}
	}
	return result
}

func linesOverlap(aStart, aEnd, bStart, bEnd int) bool {
	return aStart <= bEnd && bStart <= aEnd
}

func normalizePath(p string) string {
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "./")
	return p
}
