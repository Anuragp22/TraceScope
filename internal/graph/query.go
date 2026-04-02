package graph

// BlastRadiusResult holds the result of a blast radius computation.
type BlastRadiusResult struct {
	// AffectedNodes are the nodes within the blast radius, keyed by ID.
	AffectedNodes map[string]*Node
	// Depth records the BFS depth at which each node was reached.
	Depth map[string]int
	// Parent records the preceding node on the shortest reverse-impact path.
	Parent map[string]string
	// ParentEdge records the edge type from each node to its parent.
	ParentEdge map[string]EdgeType
	// Confidence records the weakest confidence level along the path from a seed
	// to each affected node.
	Confidence map[string]EdgeConfidence
	// MaxDepth is the maximum depth searched.
	MaxDepth int
}

// ComputeBlastRadius performs a reverse BFS from seed nodes up to maxDepth.
// It follows CALLS and CONTAINS edges in reverse (who calls this? what file contains this?).
// IMPORTS edges are excluded to avoid over-inflating the blast radius.
func ComputeBlastRadius(graphData *GraphData, seedNodeIDs []string, maxDepth int) *BlastRadiusResult {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	// Build lookup maps
	nodeMap := make(map[string]*Node, len(graphData.Nodes))
	for i := range graphData.Nodes {
		nodeMap[graphData.Nodes[i].ID] = &graphData.Nodes[i]
	}

	// Build reverse adjacency: target → list of source node IDs
	// Only for CALLS and CONTAINS — IMPORTS are excluded (M7)
	type reverseEdge struct {
		source     string
		edgeType   EdgeType
		confidence EdgeConfidence
	}
	reverseAdj := make(map[string][]reverseEdge)
	for _, edge := range graphData.Edges {
		confidence := edge.Confidence
		if confidence == "" {
			confidence = EdgeConfidenceExact
		}
		switch edge.Type {
		case EdgeCalls:
			// A calls B → if B changes, A is affected
			reverseAdj[edge.Target] = append(reverseAdj[edge.Target], reverseEdge{source: edge.Source, edgeType: edge.Type, confidence: confidence})
		case EdgeContains:
			// File contains Func → if Func changes, File is affected
			reverseAdj[edge.Target] = append(reverseAdj[edge.Target], reverseEdge{source: edge.Source, edgeType: edge.Type, confidence: confidence})
		case EdgeExtends, EdgeImplements:
			// B extends A → if A changes, B is affected
			reverseAdj[edge.Target] = append(reverseAdj[edge.Target], reverseEdge{source: edge.Source, edgeType: edge.Type, confidence: confidence})
		}
	}

	result := &BlastRadiusResult{
		AffectedNodes: make(map[string]*Node),
		Depth:         make(map[string]int),
		Parent:        make(map[string]string),
		ParentEdge:    make(map[string]EdgeType),
		Confidence:    make(map[string]EdgeConfidence),
		MaxDepth:      maxDepth,
	}

	// BFS using a slice-backed queue to avoid per-element list allocations.
	type bfsEntry struct {
		id    string
		depth int
	}

	visited := make(map[string]bool)
	queue := make([]bfsEntry, 0, len(seedNodeIDs))
	head := 0

	for _, id := range seedNodeIDs {
		if _, ok := nodeMap[id]; ok {
			visited[id] = true
			result.AffectedNodes[id] = nodeMap[id]
			result.Depth[id] = 0
			result.Confidence[id] = EdgeConfidenceExact
			queue = append(queue, bfsEntry{id: id, depth: 0})
		}
	}

	for head < len(queue) {
		curr := queue[head]
		head++

		if curr.depth >= maxDepth {
			continue
		}

		for _, neighbor := range reverseAdj[curr.id] {
			neighborID := neighbor.source
			if visited[neighborID] {
				continue
			}
			visited[neighborID] = true
			nextDepth := curr.depth + 1
			if node, ok := nodeMap[neighborID]; ok {
				result.AffectedNodes[neighborID] = node
				result.Depth[neighborID] = nextDepth
				result.Parent[neighborID] = curr.id
				result.ParentEdge[neighborID] = neighbor.edgeType
				result.Confidence[neighborID] = mergeConfidence(result.Confidence[curr.id], neighbor.confidence)
				queue = append(queue, bfsEntry{id: neighborID, depth: nextDepth})
			}
		}
	}

	return result
}

func mergeConfidence(pathConfidence, edgeConfidence EdgeConfidence) EdgeConfidence {
	if pathConfidence == EdgeConfidenceHeuristic || edgeConfidence == EdgeConfidenceHeuristic {
		return EdgeConfidenceHeuristic
	}
	return EdgeConfidenceExact
}
