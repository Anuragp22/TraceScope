package graph

import (
	"fmt"
	"sort"
	"strings"
)

// GraphComparison summarizes structural overlap between two dependency graphs.
type GraphComparison struct {
	BaseNodeCount      int      `json:"base_node_count"`
	CandidateNodeCount int      `json:"candidate_node_count"`
	SharedNodeCount    int      `json:"shared_node_count"`
	MissingNodeCount   int      `json:"missing_node_count"`
	ExtraNodeCount     int      `json:"extra_node_count"`
	BaseEdgeCount      int      `json:"base_edge_count"`
	CandidateEdgeCount int      `json:"candidate_edge_count"`
	SharedEdgeCount    int      `json:"shared_edge_count"`
	MissingEdgeCount   int      `json:"missing_edge_count"`
	ExtraEdgeCount     int      `json:"extra_edge_count"`
	MissingNodes       []string `json:"missing_nodes,omitempty"`
	ExtraNodes         []string `json:"extra_nodes,omitempty"`
	MissingEdges       []string `json:"missing_edges,omitempty"`
	ExtraEdges         []string `json:"extra_edges,omitempty"`
}

// CompareGraphShape compares candidate against base by stable node/edge signatures.
// Paths are normalized relative to root so parser and SCIP graphs can be compared
// even when one uses absolute paths and the other stores repo-relative paths.
func CompareGraphShape(base, candidate *GraphData, root string, maxExamples int) GraphComparison {
	baseNodes, baseEdges := graphSignatureSets(base, root)
	candidateNodes, candidateEdges := graphSignatureSets(candidate, root)

	comparison := GraphComparison{
		BaseNodeCount:      len(baseNodes),
		CandidateNodeCount: len(candidateNodes),
		SharedNodeCount:    countSharedSignatures(baseNodes, candidateNodes),
		MissingNodes:       diffSignatures(baseNodes, candidateNodes, maxExamples),
		ExtraNodes:         diffSignatures(candidateNodes, baseNodes, maxExamples),
		BaseEdgeCount:      len(baseEdges),
		CandidateEdgeCount: len(candidateEdges),
		SharedEdgeCount:    countSharedSignatures(baseEdges, candidateEdges),
		MissingEdges:       diffSignatures(baseEdges, candidateEdges, maxExamples),
		ExtraEdges:         diffSignatures(candidateEdges, baseEdges, maxExamples),
	}
	comparison.MissingNodeCount = comparison.BaseNodeCount - comparison.SharedNodeCount
	comparison.ExtraNodeCount = comparison.CandidateNodeCount - comparison.SharedNodeCount
	comparison.MissingEdgeCount = comparison.BaseEdgeCount - comparison.SharedEdgeCount
	comparison.ExtraEdgeCount = comparison.CandidateEdgeCount - comparison.SharedEdgeCount
	return comparison
}

func graphSignatureSets(gd *GraphData, root string) (map[string]struct{}, map[string]struct{}) {
	if gd == nil {
		return map[string]struct{}{}, map[string]struct{}{}
	}

	nodeSignatures := map[string]struct{}{}
	nodeSignatureByID := map[string]string{}
	for _, node := range gd.Nodes {
		if node.Type == NodeFunction && node.Name == "default" {
			continue
		}
		signature := graphNodeSignature(node, root)
		nodeSignatures[signature] = struct{}{}
		nodeSignatureByID[node.ID] = signature
	}

	edgeSignatures := map[string]struct{}{}
	for _, edge := range gd.Edges {
		sourceSignature := nodeSignatureByID[edge.Source]
		targetSignature := nodeSignatureByID[edge.Target]
		if sourceSignature == "" || targetSignature == "" {
			continue
		}
		edgeSignatures[fmt.Sprintf("%s -> %s [%s]", sourceSignature, targetSignature, edge.Type)] = struct{}{}
	}
	return nodeSignatures, edgeSignatures
}

func graphNodeSignature(node Node, root string) string {
	path := rootRelativeGraphPath(node.FilePath, root)
	return fmt.Sprintf("%s:%s:%s", node.Type, path, node.Name)
}

func rootRelativeGraphPath(path, root string) string {
	normalizedPath := canonicalPath(path)
	normalizedRoot := strings.TrimSuffix(canonicalPath(root), "/")
	if normalizedRoot == "" {
		return normalizedPath
	}
	if normalizedPath == normalizedRoot {
		return "."
	}
	if strings.HasPrefix(normalizedPath, normalizedRoot+"/") {
		return strings.TrimPrefix(normalizedPath, normalizedRoot+"/")
	}
	return normalizedPath
}

func countSharedSignatures(base, candidate map[string]struct{}) int {
	shared := 0
	for signature := range base {
		if _, exists := candidate[signature]; exists {
			shared++
		}
	}
	return shared
}

func diffSignatures(base, candidate map[string]struct{}, maxExamples int) []string {
	diff := make([]string, 0)
	for signature := range base {
		if _, exists := candidate[signature]; !exists {
			diff = append(diff, signature)
		}
	}
	sort.Strings(diff)
	if maxExamples > 0 && len(diff) > maxExamples {
		return diff[:maxExamples]
	}
	return diff
}
