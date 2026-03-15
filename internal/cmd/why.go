package cmd

import (
	"fmt"

	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/output"
	"github.com/spf13/cobra"
)

var (
	whyFormat  string
	whyReverse bool
)

var whyCmd = &cobra.Command{
	Use:   "why <functionA> <functionB>",
	Short: "Show the shortest call path between two functions",
	Long: `Find how function A depends on function B through the call graph.

Examples:
  tracescope why main Score
  tracescope why graph.Build analyzer.Score
  tracescope why Score main --reverse`,
	Args:          cobra.ExactArgs(2),
	RunE:          runWhy,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	whyCmd.Flags().StringVar(&whyFormat, "format", "terminal", "output format: terminal or json")
	whyCmd.Flags().BoolVar(&whyReverse, "reverse", false, "traverse call edges in reverse (who calls whom)")
	rootCmd.AddCommand(whyCmd)
}

func runWhy(cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Changed("format") && cfg.Format != "" {
		whyFormat = cfg.Format
	}

	graphData, err := loadGraph()
	if err != nil {
		return fmt.Errorf("loading graph (run 'tracescope index' first): %w", err)
	}

	// Resolve function A
	sourceNode, err := resolveFunction(graphData, args[0])
	if err != nil {
		return err
	}

	// Resolve function B
	targetNode, err := resolveFunction(graphData, args[1])
	if err != nil {
		return err
	}

	// Find shortest path
	result := graph.FindShortestPath(graphData, sourceNode.ID, targetNode.ID, whyReverse)

	switch whyFormat {
	case "json":
		if err := output.PrintJSON(result); err != nil {
			return fmt.Errorf("writing JSON: %w", err)
		}
	default:
		output.PrintPath(result)
	}

	return nil
}

// resolveFunction finds a single function node from a partial name.
// Returns an error if no match or if ambiguous (showing candidates).
func resolveFunction(graphData *graph.GraphData, query string) (*graph.Node, error) {
	matches := graph.FindNodesByName(graphData, query)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no function matching %q found in the graph", query)
	}

	// If there's exactly one match, use it
	if len(matches) == 1 {
		return matches[0].Node, nil
	}

	// If first match is exact and unique name, use it
	if matches[0].MatchType == "exact" {
		// Check if there are multiple exact matches (same name, different packages)
		exactCount := 0
		for _, m := range matches {
			if m.MatchType == "exact" {
				exactCount++
			}
		}
		if exactCount == 1 {
			return matches[0].Node, nil
		}
	}

	// If first match is qualified, it's specific enough
	if matches[0].MatchType == "qualified" {
		return matches[0].Node, nil
	}

	// Ambiguous — show candidates (limit to 10)
	shown := matches
	if len(shown) > 10 {
		shown = shown[:10]
	}
	output.PrintNodeMatches(query, shown)
	return nil, fmt.Errorf("ambiguous function name %q — be more specific", query)
}
