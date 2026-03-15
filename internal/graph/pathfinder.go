package graph

import (
	"container/list"
	"strings"
)

// PathStep represents one node in a call path.
type PathStep struct {
	Node     *Node  `json:"node"`
	EdgeType string `json:"edge_type,omitempty"`
}

// PathResult holds the result of a shortest-path query.
type PathResult struct {
	Found   bool       `json:"found"`
	Path    []PathStep `json:"path,omitempty"`
	Source  *Node      `json:"source"`
	Target  *Node      `json:"target"`
	Length  int        `json:"length"`
	Message string     `json:"message,omitempty"`
}

// NodeMatch represents a candidate match for a partial name query.
type NodeMatch struct {
	Node       *Node  `json:"node"`
	MatchType  string `json:"match_type"` // "exact", "qualified", "prefix", "substring"
	Qualifier  string `json:"qualifier"`  // e.g., "graph.Build"
}

// FindNodesByName searches function nodes by partial name.
// Returns matches ranked by specificity: exact > qualified > prefix > substring.
func FindNodesByName(graphData *GraphData, query string) []NodeMatch {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	var exact, qualified, prefix, substring []NodeMatch

	// Check if query contains a qualifier (e.g., "graph.Build")
	queryPkg, queryName := "", query
	if idx := strings.LastIndex(query, "."); idx != -1 {
		queryPkg = query[:idx]
		queryName = query[idx+1:]
	}

	for i := range graphData.Nodes {
		n := &graphData.Nodes[i]
		if n.Type != NodeFunction {
			continue
		}

		qualifier := n.Package + "." + n.Name

		// Exact name match
		if n.Name == query {
			exact = append(exact, NodeMatch{Node: n, MatchType: "exact", Qualifier: qualifier})
			continue
		}

		// Qualified match: "graph.Build" matches package="graph", name="Build"
		if queryPkg != "" && n.Name == queryName {
			if n.Package == queryPkg || strings.HasSuffix(n.Package, "/"+queryPkg) {
				qualified = append(qualified, NodeMatch{Node: n, MatchType: "qualified", Qualifier: qualifier})
				continue
			}
		}

		// Prefix match: "Build" matches "BuildGraph"
		if strings.HasPrefix(n.Name, query) {
			prefix = append(prefix, NodeMatch{Node: n, MatchType: "prefix", Qualifier: qualifier})
			continue
		}

		// Case-insensitive exact match
		if strings.EqualFold(n.Name, query) {
			substring = append(substring, NodeMatch{Node: n, MatchType: "substring", Qualifier: qualifier})
			continue
		}

		// Substring match: "build" matches "RebuildGraph"
		if strings.Contains(strings.ToLower(n.Name), strings.ToLower(query)) {
			substring = append(substring, NodeMatch{Node: n, MatchType: "substring", Qualifier: qualifier})
		}
	}

	// Return in order of specificity
	var result []NodeMatch
	result = append(result, exact...)
	result = append(result, qualified...)
	result = append(result, prefix...)
	result = append(result, substring...)
	return result
}

// FindShortestPath finds the shortest call path between two nodes using BFS.
// If reverse is true, traverses edges backwards (target → source).
func FindShortestPath(graphData *GraphData, sourceID, targetID string, reverse bool) *PathResult {
	nodeMap := make(map[string]*Node, len(graphData.Nodes))
	for i := range graphData.Nodes {
		nodeMap[graphData.Nodes[i].ID] = &graphData.Nodes[i]
	}

	srcNode := nodeMap[sourceID]
	tgtNode := nodeMap[targetID]

	if srcNode == nil || tgtNode == nil {
		return &PathResult{
			Found:   false,
			Source:  srcNode,
			Target:  tgtNode,
			Message: "one or both nodes not found in graph",
		}
	}

	// Self-reference
	if sourceID == targetID {
		return &PathResult{
			Found:  true,
			Source: srcNode,
			Target: tgtNode,
			Path:   []PathStep{{Node: srcNode}},
			Length: 0,
		}
	}

	// Build adjacency map for CALLS edges
	adj := make(map[string][]string)
	edgeTypes := make(map[string]EdgeType) // "source->target" → edge type
	for _, e := range graphData.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		if reverse {
			adj[e.Target] = append(adj[e.Target], e.Source)
		} else {
			adj[e.Source] = append(adj[e.Source], e.Target)
		}
		edgeTypes[e.Source+"->"+e.Target] = e.Type
	}

	// BFS
	type bfsEntry struct {
		id string
	}

	visited := make(map[string]bool)
	parent := make(map[string]string) // child → parent
	visited[sourceID] = true

	queue := list.New()
	queue.PushBack(bfsEntry{sourceID})

	found := false
	for queue.Len() > 0 {
		front := queue.Front()
		curr := front.Value.(bfsEntry)
		queue.Remove(front)

		for _, neighborID := range adj[curr.id] {
			if visited[neighborID] {
				continue
			}
			visited[neighborID] = true
			parent[neighborID] = curr.id

			if neighborID == targetID {
				found = true
				break
			}
			queue.PushBack(bfsEntry{neighborID})
		}
		if found {
			break
		}
	}

	if !found {
		direction := "forward"
		if reverse {
			direction = "reverse"
		}
		return &PathResult{
			Found:   false,
			Source:  srcNode,
			Target:  tgtNode,
			Message: "no " + direction + " call path exists between these functions",
		}
	}

	// Reconstruct path
	var pathIDs []string
	for id := targetID; id != ""; id = parent[id] {
		pathIDs = append(pathIDs, id)
	}
	// Reverse to get source → target order
	for i, j := 0, len(pathIDs)-1; i < j; i, j = i+1, j-1 {
		pathIDs[i], pathIDs[j] = pathIDs[j], pathIDs[i]
	}

	steps := make([]PathStep, len(pathIDs))
	for i, id := range pathIDs {
		edgeType := ""
		if i < len(pathIDs)-1 {
			from, to := id, pathIDs[i+1]
			if reverse {
				from, to = to, from
			}
			if et, ok := edgeTypes[from+"->"+to]; ok {
				edgeType = string(et)
			} else {
				edgeType = "CALLS"
			}
		}
		steps[i] = PathStep{
			Node:     nodeMap[id],
			EdgeType: edgeType,
		}
	}

	return &PathResult{
		Found:  true,
		Source: srcNode,
		Target: tgtNode,
		Path:   steps,
		Length: len(steps) - 1,
	}
}
