package analyzer

import (
	"fmt"
	"math"
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
	Node         *graph.Node          `json:"node"`
	Depth        int                  `json:"depth"`
	Risk         RiskLevel            `json:"risk"`
	ReviewScore  int                  `json:"review_score,omitempty"`
	Confidence   graph.EdgeConfidence `json:"confidence,omitempty"`
	CallerCount  int                  `json:"caller_count"`
	Reason       string               `json:"reason"`
	ImpactPath   []graph.PathStep     `json:"impact_path,omitempty"`
	LastAuthor   string               `json:"last_author,omitempty"`
	LastEmail    string               `json:"last_email,omitempty"`
	LastModified string               `json:"last_modified,omitempty"`
}

// AnalysisResult holds the complete blast radius analysis.
type AnalysisResult struct {
	ChangedFunctions  []ChangedFunction       `json:"changed_functions"`
	AffectedFunctions []AffectedFunction      `json:"affected_functions"`
	ChangedFiles      []diff.ChangedFile      `json:"changed_files"`
	ResolutionStats   graph.ResolutionStats   `json:"resolution_stats,omitempty"`
	ResolutionIssues  []graph.ResolutionIssue `json:"resolution_issues,omitempty"`
	TotalNodes        int                     `json:"total_nodes"`
	TotalEdges        int                     `json:"total_edges"`
	MaxDepth          int                     `json:"max_depth"`
	TotalAffected     int                     `json:"total_affected,omitempty"`
	TopN              int                     `json:"top_n,omitempty"`
	Ownership         interface{}             `json:"ownership,omitempty"`
}

// RiskExitError is returned when the analysis completes but risk was found.
type RiskExitError struct {
	Code int
}

func (e *RiskExitError) Error() string {
	return fmt.Sprintf("risk exit code: %d", e.Code)
}

// HighestRiskExitCode returns 1 for HIGH, 2 for MEDIUM, 0 for LOW/none.
func HighestRiskExitCode(result *AnalysisResult) int {
	for _, af := range result.AffectedFunctions {
		if af.Risk == RiskHigh {
			return 1
		}
	}
	for _, af := range result.AffectedFunctions {
		if af.Risk == RiskMedium {
			return 2
		}
	}
	return 0
}

// BlastRadiusAnalyzer orchestrates the full analysis pipeline.
type BlastRadiusAnalyzer struct {
	graphData *graph.GraphData
	maxDepth  int
	scorer    *RiskScorer
}

// NewBlastRadiusAnalyzer creates a new analyzer with a configurable max depth and scorer.
// If scorer is nil, a default scorer is used.
func NewBlastRadiusAnalyzer(graphData *graph.GraphData, maxDepth int, scorer *RiskScorer) *BlastRadiusAnalyzer {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	if scorer == nil {
		scorer = &RiskScorer{}
	}
	return &BlastRadiusAnalyzer{graphData: graphData, maxDepth: maxDepth, scorer: scorer}
}

// Analyze runs the full analysis: diff → functions → blast radius → risk scoring.
func (a *BlastRadiusAnalyzer) Analyze(changedFiles []diff.ChangedFile) *AnalysisResult {
	result := &AnalysisResult{
		ChangedFiles:     changedFiles,
		ResolutionStats:  a.graphData.ResolutionStats,
		ResolutionIssues: a.graphData.ResolutionIssues,
		TotalNodes:       len(a.graphData.Nodes),
		TotalEdges:       len(a.graphData.Edges),
		MaxDepth:         a.maxDepth,
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
	nodeMap := buildNodeLookup(a.graphData)

	// Step 5: Score risk for affected nodes
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
		risk, reason := a.scorer.Score(node, depth, prodCount)

		result.AffectedFunctions = append(result.AffectedFunctions, AffectedFunction{
			Node:        node,
			Depth:       depth,
			Risk:        risk,
			ReviewScore: computeReviewScore(node, risk, br.Confidence[id], prodCount, depth),
			Confidence:  br.Confidence[id],
			CallerCount: count,
			Reason:      reason,
			ImpactPath:  buildImpactPath(id, br.Parent, br.ParentEdge, nodeMap),
		})
	}

	// Sort for deterministic output: risk level → callers desc → name asc
	sort.Slice(result.AffectedFunctions, func(i, j int) bool {
		if result.AffectedFunctions[i].ReviewScore != result.AffectedFunctions[j].ReviewScore {
			return result.AffectedFunctions[i].ReviewScore > result.AffectedFunctions[j].ReviewScore
		}
		ri, rj := riskOrder(result.AffectedFunctions[i].Risk), riskOrder(result.AffectedFunctions[j].Risk)
		if ri != rj {
			return ri < rj
		}
		if result.AffectedFunctions[i].CallerCount != result.AffectedFunctions[j].CallerCount {
			return result.AffectedFunctions[i].CallerCount > result.AffectedFunctions[j].CallerCount
		}
		if result.AffectedFunctions[i].Depth != result.AffectedFunctions[j].Depth {
			return result.AffectedFunctions[i].Depth < result.AffectedFunctions[j].Depth
		}
		if result.AffectedFunctions[i].Node.Name != result.AffectedFunctions[j].Node.Name {
			return result.AffectedFunctions[i].Node.Name < result.AffectedFunctions[j].Node.Name
		}
		// Node.ID is unique — final tiebreaker for fully deterministic output.
		return result.AffectedFunctions[i].Node.ID < result.AffectedFunctions[j].Node.ID
	})

	return result
}

func buildImpactPath(nodeID string, parent map[string]string, parentEdge map[string]graph.EdgeType, nodeMap map[string]*graph.Node) []graph.PathStep {
	var reversed []graph.PathStep
	for id := nodeID; id != ""; id = parent[id] {
		node := nodeMap[id]
		if node == nil {
			break
		}
		step := graph.PathStep{Node: node}
		if edgeType, ok := parentEdge[id]; ok {
			step.EdgeType = string(edgeType)
		}
		reversed = append(reversed, step)
	}

	if len(reversed) <= 1 {
		return nil
	}

	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return reversed
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

// callerGranularityWeight scales the log1p caller term. Chosen so that within a
// tier the spread (~0 at 0 callers up to ~50 at 600) refines ordering without a
// single hub crossing into the next tier's base band (tiers are 30 apart).
const callerGranularityWeight = 8.0

// computeReviewScore turns a scored affected function into an integer ranking key.
//
// It is deliberately NOT a re-derivation of the risk tier. The tier base
// (80/50/20) already encodes the coarse category, which the ladder derives from
// production caller count and export status. Two mistakes the previous version
// made — re-adding a linear caller bonus and a separate +10 for exported — double
// counted that same evidence, and the linear bonus saturated so a 6- and a
// 600-caller function tied. Here the tier base carries the category; a log1p
// caller term adds *within-tier* granularity (uncapped, sub-linear); and only
// signals orthogonal to the tier — impact proximity, whether the node itself is
// test code, and path confidence — adjust the score further.
func computeReviewScore(node *graph.Node, risk RiskLevel, confidence graph.EdgeConfidence, prodCallerCount, depth int) int {
	score := 0
	switch risk {
	case RiskHigh:
		score += 80
	case RiskMedium:
		score += 50
	case RiskLow:
		score += 20
	}

	// Within-tier granularity. Only production callers count — a function called
	// widely from tests is not a review priority. log1p keeps growing so 600
	// callers outranks 6, but sub-linearly so no single hub dominates.
	score += int(math.Round(callerGranularityWeight * math.Log1p(float64(prodCallerCount))))

	// Orthogonal signals the tier does not encode.
	if !node.IsTest {
		score += 6 // a test function in the blast radius is a lower review priority
	}
	switch {
	case depth <= 1:
		score += 12 // a direct dependency is closer to the change
	case depth == 2:
		score += 6
	case depth >= 4:
		score -= 4 // far-away impact is weaker evidence
	}
	if confidence == graph.EdgeConfidenceHeuristic {
		score -= 8 // a heuristic path is less trustworthy than an exact one
	}

	if score < 0 {
		return 0
	}
	return score
}

// buildCallerCountMaps counts total and production (non-test) callers for each function.
func buildCallerCountMaps(gd *graph.GraphData) (map[string]int, map[string]int) {
	nodeMap := buildNodeLookup(gd)

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

func buildNodeLookup(gd *graph.GraphData) map[string]*graph.Node {
	nodeMap := make(map[string]*graph.Node, len(gd.Nodes))
	for i := range gd.Nodes {
		nodeMap[gd.Nodes[i].ID] = &gd.Nodes[i]
	}
	return nodeMap
}
