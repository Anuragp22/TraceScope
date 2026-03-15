package analyzer

import (
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
	Node       *graph.Node
	Depth      int
	Risk       RiskLevel
	CallerCount int
	Reason     string
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
}

// NewBlastRadiusAnalyzer creates a new analyzer.
func NewBlastRadiusAnalyzer(graphData *graph.GraphData) *BlastRadiusAnalyzer {
	return &BlastRadiusAnalyzer{graphData: graphData}
}

// Analyze runs the full analysis: diff → functions → blast radius → risk scoring.
func (a *BlastRadiusAnalyzer) Analyze(changedFiles []diff.ChangedFile) *AnalysisResult {
	result := &AnalysisResult{
		ChangedFiles: changedFiles,
		TotalNodes:   len(a.graphData.Nodes),
		TotalEdges:   len(a.graphData.Edges),
		MaxDepth:     5,
	}

	// Step 1: Map diff to changed functions
	changedFuncs := MapDiffToFunctions(changedFiles, a.graphData)
	result.ChangedFunctions = changedFuncs

	// Also get changed file node IDs
	changedFileIDs := MapDiffToFileNodes(changedFiles, a.graphData)

	// Step 2: Collect seed node IDs
	seedIDs := make([]string, 0, len(changedFuncs)+len(changedFileIDs))
	seedSet := make(map[string]bool)
	for _, cf := range changedFuncs {
		if !seedSet[cf.NodeID] {
			seedIDs = append(seedIDs, cf.NodeID)
			seedSet[cf.NodeID] = true
		}
	}
	for _, fid := range changedFileIDs {
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

	// Step 4: Build caller count map
	callerCount := buildCallerCountMap(a.graphData)

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
		risk, reason := scorer.Score(node, count)

		result.AffectedFunctions = append(result.AffectedFunctions, AffectedFunction{
			Node:        node,
			Depth:       depth,
			Risk:        risk,
			CallerCount: count,
			Reason:      reason,
		})
	}

	return result
}

// buildCallerCountMap counts how many functions call each function.
func buildCallerCountMap(gd *graph.GraphData) map[string]int {
	counts := make(map[string]int)
	for _, e := range gd.Edges {
		if e.Type == graph.EdgeCalls {
			counts[e.Target]++
		}
	}
	return counts
}
