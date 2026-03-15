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
	seen := make(map[string]bool) // deduplicate by NodeID

	for _, cf := range changedFiles {
		normDiffPath := normalizePath(cf.Path)

		// Try to match the diff path to a known file using path-segment matching
		var matchedPath string
		for knownPath := range funcsByFile {
			if knownPath == normDiffPath || pathSegmentSuffix(knownPath, normDiffPath) || pathSegmentSuffix(normDiffPath, knownPath) {
				matchedPath = knownPath
				break
			}
		}

		if matchedPath == "" {
			continue
		}

		funcs := funcsByFile[matchedPath]
		for _, fn := range funcs {
			if seen[fn.ID] {
				continue
			}

			if cf.IsNew {
				// New file — all functions are changed
				seen[fn.ID] = true
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
					seen[fn.ID] = true
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
			if knownPath == normPath || pathSegmentSuffix(knownPath, normPath) || pathSegmentSuffix(normPath, knownPath) {
				result = append(result, id)
				break
			}
		}
	}
	return result
}

// pathSegmentSuffix checks if path ends with suffix when compared by path segments.
// e.g., pathSegmentSuffix("src/utils/helper.go", "utils/helper.go") → true
// but pathSegmentSuffix("src/myutils/helper.go", "utils/helper.go") → false
func pathSegmentSuffix(path, suffix string) bool {
	pathParts := strings.Split(path, "/")
	suffixParts := strings.Split(suffix, "/")
	if len(suffixParts) > len(pathParts) {
		return false
	}
	for i := range suffixParts {
		if pathParts[len(pathParts)-len(suffixParts)+i] != suffixParts[i] {
			return false
		}
	}
	return true
}

func linesOverlap(aStart, aEnd, bStart, bEnd int) bool {
	return aStart <= bEnd && bStart <= aEnd
}

func normalizePath(p string) string {
	p = filepath.ToSlash(filepath.Clean(p))
	p = strings.TrimPrefix(p, "./")
	return p
}
