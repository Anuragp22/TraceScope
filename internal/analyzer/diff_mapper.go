package analyzer

import (
	"path/filepath"
	"strings"

	"github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
)

// ChangedFunction represents a function that was modified in the diff.
type ChangedFunction struct {
	NodeID   string      `json:"node_id"`
	Node     *graph.Node `json:"node"`
	FilePath string      `json:"file_path"`
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

	funcFileKeys := make([]string, 0, len(funcsByFile))
	for k := range funcsByFile {
		funcFileKeys = append(funcFileKeys, k)
	}

	var result []ChangedFunction
	seen := make(map[string]bool) // deduplicate by NodeID

	for _, cf := range changedFiles {
		normDiffPath := normalizePath(cf.Path)

		// Deterministically pick the best-matching graph file for this diff path.
		matchedPath := matchGraphPath(normDiffPath, funcFileKeys)
		if matchedPath == "" {
			continue
		}

		funcs := funcsByFile[matchedPath]
		for _, fn := range funcs {
			if seen[fn.ID] {
				continue
			}

			if cf.IsNew || cf.IsDeleted {
				// New file — every function is added. Deleted file — every
				// function is removed, so its callers still need traversal.
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

	fileNodeKeys := make([]string, 0, len(fileNodes))
	for k := range fileNodes {
		fileNodeKeys = append(fileNodeKeys, k)
	}

	var result []string
	for _, cf := range changedFiles {
		normPath := normalizePath(cf.Path)
		if matched := matchGraphPath(normPath, fileNodeKeys); matched != "" {
			result = append(result, fileNodes[matched])
		}
	}
	return result
}

// matchGraphPath picks the graph file path that best matches a diff path.
// It prefers an exact match, then the longest path-segment-suffix match,
// breaking remaining ties alphabetically so the result is deterministic
// regardless of map iteration order.
func matchGraphPath(diffPath string, candidates []string) string {
	best := ""
	for _, p := range candidates {
		if p == diffPath {
			return p
		}
		if !pathSegmentSuffix(p, diffPath) && !pathSegmentSuffix(diffPath, p) {
			continue
		}
		if best == "" || len(p) > len(best) || (len(p) == len(best) && p < best) {
			best = p
		}
	}
	return best
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
