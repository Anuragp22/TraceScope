package analyzer

import (
	"sort"

	"github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
)

// RiskLevel represents the risk level of an affected function.
type RiskLevel string

const (
	RiskHigh   RiskLevel = "HIGH"
	RiskMedium RiskLevel = "MEDIUM"
	RiskLow    RiskLevel = "LOW"
)

// AffectedFunction is a function in the blast radius with its risk assessment.
type AffectedFunction struct {
	Node        *graph.Node
	Depth       int
	Risk        RiskLevel
	CallerCount int
	Reason      string
}

// AnalysisResult holds the complete blast radius analysis.
type AnalysisResult struct {
	ChangedFunctions  []ChangedFunction
	AffectedFunctions []AffectedFunction
	ChangedFiles      []diff.ChangedFile
	TotalNodes        int
	TotalEdges        int
	MaxDepth          int
}

// BlastRadiusAnalyzer orchestrates the full analysis pipeline.
type BlastRadiusAnalyzer struct {
	graphData *graph.GraphData
	maxDepth  int
}

// NewBlastRadiusAnalyzer creates a new analyzer with a configurable max depth.
func NewBlastRadiusAnalyzer(graphData *graph.GraphData, maxDepth int) *BlastRadiusAnalyzer {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	return &BlastRadiusAnalyzer{graphData: graphData, maxDepth: maxDepth}
}

// Analyze runs the full analysis: diff → functions → blast radius → risk scoring.
func (a *BlastRadiusAnalyzer) Analyze(changedFiles []diff.ChangedFile) *AnalysisResult {
	result := &AnalysisResult{
		ChangedFiles: changedFiles,
		TotalNodes:   len(a.graphData.Nodes),
		TotalEdges:   len(a.graphData.Edges),
		MaxDepth:     a.maxDepth,
	}

	// Step 1: Map diff to changed functions
	changedFuncs := MapDiffToFunctions(changedFiles, a.graphData)
	result.ChangedFunctions = changedFuncs

	// Also get changed file node IDs
	changedFileIDs := MapDiffToFileNodes(changedFiles, a.graphData)

	// Step 2: Collect seed node IDs — prefer function nodes, only add file
	// nodes for files that had no function-level matches (avoids double-counting)
	seedIDs := make([]string, 0, len(changedFuncs)+len(changedFileIDs))
	seedSet := make(map[string]bool)

	changedFuncFiles := make(map[string]bool)
	for _, cf := range changedFuncs {
		if !seedSet[cf.NodeID] {
			seedIDs = append(seedIDs, cf.NodeID)
			seedSet[cf.NodeID] = true
		}
		changedFuncFiles[cf.FilePath] = true
	}

	// Build file node lookup
	fileNodeLookup := make(map[string]string) // node ID → file path
	for _, n := range a.graphData.Nodes {
		if n.Type == graph.NodeFile {
			fileNodeLookup[n.ID] = n.FilePath
		}
	}

	for _, fid := range changedFileIDs {
		// Only add file node if no functions were found in that file
		if fp, ok := fileNodeLookup[fid]; ok && changedFuncFiles[fp] {
			continue
		}
		if !seedSet[fid] {
			seedIDs = append(seedIDs, fid)
			seedSet[fid] = true
		}
	}

	if len(seedIDs) == 0 {
		return result
	}

	// Step 3: Compute blast radius
	br := graph.ComputeBlastRadius(a.graphData, seedIDs, result.MaxDepth)

	// Step 4: Build caller count maps (total and production-only)
	callerCount, prodCallerCount := buildCallerCountMaps(a.graphData)

	// Step 5: Score risk for affected nodes
	scorer := &RiskScorer{}
	for id, node := range br.AffectedNodes {
		if seedSet[id] {
			continue // skip seeds themselves
		}
		if node.Type != graph.NodeFunction {
			continue // only score functions
		}

		depth := br.Depth[id]
		count := callerCount[id]
		prodCount := prodCallerCount[id]
		risk, reason := scorer.Score(node, count, depth, prodCount)

		result.AffectedFunctions = append(result.AffectedFunctions, AffectedFunction{
			Node:        node,
			Depth:       depth,
			Risk:        risk,
			CallerCount: count,
			Reason:      reason,
		})
	}

	// Sort for deterministic output: risk level → callers desc → name asc
	sort.Slice(result.AffectedFunctions, func(i, j int) bool {
		ri, rj := riskOrder(result.AffectedFunctions[i].Risk), riskOrder(result.AffectedFunctions[j].Risk)
		if ri != rj {
			return ri < rj
		}
		if result.AffectedFunctions[i].CallerCount != result.AffectedFunctions[j].CallerCount {
			return result.AffectedFunctions[i].CallerCount > result.AffectedFunctions[j].CallerCount
		}
		return result.AffectedFunctions[i].Node.Name < result.AffectedFunctions[j].Node.Name
	})

	return result
}

func riskOrder(r RiskLevel) int {
	switch r {
	case RiskHigh:
		return 0
	case RiskMedium:
		return 1
	case RiskLow:
		return 2
	}
	return 3
}

// buildCallerCountMaps counts total and production (non-test) callers for each function.
func buildCallerCountMaps(gd *graph.GraphData) (map[string]int, map[string]int) {
	nodeMap := make(map[string]*graph.Node, len(gd.Nodes))
	for i := range gd.Nodes {
		nodeMap[gd.Nodes[i].ID] = &gd.Nodes[i]
	}

	total := make(map[string]int)
	prod := make(map[string]int)
	for _, e := range gd.Edges {
		if e.Type == graph.EdgeCalls {
			total[e.Target]++
			if caller, ok := nodeMap[e.Source]; ok && !caller.IsTest {
				prod[e.Target]++
			}
		}
	}
	return total, prod
}
