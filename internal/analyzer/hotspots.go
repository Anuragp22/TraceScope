package analyzer

import (
	"sort"

	"github.com/anurag/tracescope/internal/graph"
)

// HotspotFunction represents a function ranked by how coupled it is.
type HotspotFunction struct {
	Node           *graph.Node `json:"node"`
	InboundCallers int         `json:"inbound_callers"`
	OutboundCalls  int         `json:"outbound_calls"`
	CouplingScore  int         `json:"coupling_score"`
	Risk           RiskLevel   `json:"risk"`
}

// HotspotsResult holds the ranked list of hotspot functions.
type HotspotsResult struct {
	Hotspots   []HotspotFunction `json:"hotspots"`
	TotalNodes int               `json:"total_nodes"`
	TotalEdges int               `json:"total_edges"`
	TopN       int               `json:"top_n,omitempty"`
	Total      int               `json:"total,omitempty"`
}

// HotspotsOptions controls filtering for hotspot analysis.
type HotspotsOptions struct {
	TopN       int
	Languages  map[string]bool // nil means all
	IgnoreGlobs []string       // file paths to exclude
}

// ComputeHotspots finds the most coupled functions in the graph.
func ComputeHotspots(graphData *graph.GraphData, opts HotspotsOptions) *HotspotsResult {
	// Build caller/callee counts from CALLS edges
	inbound := make(map[string]int)
	outbound := make(map[string]int)
	for _, e := range graphData.Edges {
		if e.Type == graph.EdgeCalls {
			inbound[e.Target]++
			outbound[e.Source]++
		}
	}

	// Build node map
	nodeMap := make(map[string]*graph.Node, len(graphData.Nodes))
	for i := range graphData.Nodes {
		nodeMap[graphData.Nodes[i].ID] = &graphData.Nodes[i]
	}

	var hotspots []HotspotFunction

	for i := range graphData.Nodes {
		n := &graphData.Nodes[i]
		if n.Type != graph.NodeFunction {
			continue
		}

		// Language filter
		if opts.Languages != nil && !opts.Languages[n.Language] {
			continue
		}

		// Ignore filter
		if len(opts.IgnoreGlobs) > 0 && matchesAnyIgnore(n.FilePath, opts.IgnoreGlobs) {
			continue
		}

		in := inbound[n.ID]
		out := outbound[n.ID]

		// Skip functions with no connections
		if in == 0 && out == 0 {
			continue
		}

		coupling := in * out
		risk := classifyHotspotRisk(in)

		hotspots = append(hotspots, HotspotFunction{
			Node:           n,
			InboundCallers: in,
			OutboundCalls:  out,
			CouplingScore:  coupling,
			Risk:           risk,
		})
	}

	// Sort: coupling desc → inbound desc → name asc
	sort.Slice(hotspots, func(i, j int) bool {
		if hotspots[i].CouplingScore != hotspots[j].CouplingScore {
			return hotspots[i].CouplingScore > hotspots[j].CouplingScore
		}
		if hotspots[i].InboundCallers != hotspots[j].InboundCallers {
			return hotspots[i].InboundCallers > hotspots[j].InboundCallers
		}
		return hotspots[i].Node.Name < hotspots[j].Node.Name
	})

	result := &HotspotsResult{
		TotalNodes: len(graphData.Nodes),
		TotalEdges: len(graphData.Edges),
	}

	total := len(hotspots)
	if opts.TopN > 0 && opts.TopN < total {
		result.Total = total
		result.TopN = opts.TopN
		hotspots = hotspots[:opts.TopN]
	}
	result.Hotspots = hotspots

	return result
}

func classifyHotspotRisk(inboundCallers int) RiskLevel {
	if inboundCallers >= 10 {
		return RiskHigh
	}
	if inboundCallers >= 5 {
		return RiskMedium
	}
	return RiskLow
}

// matchesAnyIgnore checks if a file path matches any ignore pattern.
// Uses the same ** prefix match convention as the CLI --ignore flag.
func matchesAnyIgnore(filePath string, globs []string) bool {
	for _, g := range globs {
		if g == "" {
			continue
		}
		// Handle ** as prefix match
		if len(g) > 3 && g[len(g)-3:] == "/**" {
			prefix := g[:len(g)-3]
			normalized := toSlash(filePath)
			if len(normalized) > len(prefix) && normalized[:len(prefix)+1] == prefix+"/" {
				return true
			}
		}
	}
	return false
}

func toSlash(p string) string {
	result := make([]byte, len(p))
	for i := 0; i < len(p); i++ {
		if p[i] == '\\' {
			result[i] = '/'
		} else {
			result[i] = p[i]
		}
	}
	return string(result)
}
