package graph

import (
	"container/list"
)

// BlastRadiusResult holds the result of a blast radius computation.
type BlastRadiusResult struct {
	// AffectedNodes are the nodes within the blast radius, keyed by ID.
	AffectedNodes map[string]*Node
	// Depth records the BFS depth at which each node was reached.
	Depth map[string]int
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
	reverseAdj := make(map[string][]string)
	for _, edge := range graphData.Edges {
		switch edge.Type {
		case EdgeCalls:
			// A calls B → if B changes, A is affected
			reverseAdj[edge.Target] = append(reverseAdj[edge.Target], edge.Source)
		case EdgeContains:
			// File contains Func → if Func changes, File is affected
			reverseAdj[edge.Target] = append(reverseAdj[edge.Target], edge.Source)
		case EdgeExtends, EdgeImplements:
			// B extends A → if A changes, B is affected
			reverseAdj[edge.Target] = append(reverseAdj[edge.Target], edge.Source)
		}
	}

	result := &BlastRadiusResult{
		AffectedNodes: make(map[string]*Node),
		Depth:         make(map[string]int),
		MaxDepth:      maxDepth,
	}

	// BFS using container/list for efficient queue operations
	type bfsEntry struct {
		id    string
		depth int
	}

	visited := make(map[string]bool)
	queue := list.New()

	for _, id := range seedNodeIDs {
		if _, ok := nodeMap[id]; ok {
			visited[id] = true
			result.AffectedNodes[id] = nodeMap[id]
			result.Depth[id] = 0
			queue.PushBack(bfsEntry{id, 0})
		}
	}

	for queue.Len() > 0 {
		front := queue.Front()
		curr := front.Value.(bfsEntry)
		queue.Remove(front)

		if curr.depth >= maxDepth {
			continue
		}

		for _, neighborID := range reverseAdj[curr.id] {
			if visited[neighborID] {
				continue
			}
			visited[neighborID] = true
			nextDepth := curr.depth + 1
			if node, ok := nodeMap[neighborID]; ok {
				result.AffectedNodes[neighborID] = node
				result.Depth[neighborID] = nextDepth
				queue.PushBack(bfsEntry{neighborID, nextDepth})
			}
		}
	}

	return result
}
