package cmd

import (
	"fmt"

	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/output"
	"github.com/spf13/cobra"
)

var (
	hotspotsTopN   int
	hotspotsFormat string
	hotspotsLang   string
	hotspotsIgnore string
)

var hotspotsCmd = &cobra.Command{
	Use:           "hotspots",
	Short:         "Show the most coupled functions in the codebase",
	Long: `Ranks functions by coupling score to identify fragile code.

The score is (inbound callers × 2) + outbound calls. Inbound is weighted
because it is the blast radius if the function changes. It is a weighted sum
rather than a product so that a heavily called function with no outbound calls
of its own does not score zero.`,
	RunE:          runHotspots,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	hotspotsCmd.Flags().IntVar(&hotspotsTopN, "top", 10, "number of hotspots to show")
	hotspotsCmd.Flags().StringVar(&hotspotsFormat, "format", "terminal", "output format: terminal or json")
	hotspotsCmd.Flags().StringVar(&hotspotsLang, "lang", "", "filter by language (e.g., go,js,py)")
	hotspotsCmd.Flags().StringVar(&hotspotsIgnore, "ignore", "", "comma-separated glob patterns to exclude files")
	rootCmd.AddCommand(hotspotsCmd)
}

func runHotspots(cmd *cobra.Command, args []string) error {
	// Merge config
	if !cmd.Flags().Changed("top") && cfg.TopN > 0 {
		hotspotsTopN = cfg.TopN
	}
	if !cmd.Flags().Changed("format") && cfg.Format != "" {
		hotspotsFormat = cfg.Format
	}

	// Build ignore globs from config + CLI
	var ignoreGlobList []string
	if len(cfg.Ignore) > 0 {
		ignoreGlobList = append(ignoreGlobList, cfg.Ignore...)
	}
	if hotspotsIgnore != "" {
		ignoreGlobList = append(ignoreGlobList, splitGlobs(hotspotsIgnore)...)
	}

	graphData, err := loadGraph()
	if err != nil {
		return fmt.Errorf("loading graph (run 'tracescope index' first): %w", err)
	}

	opts := analyzer.HotspotsOptions{
		TopN:        hotspotsTopN,
		Languages:   parseLangFilter(hotspotsLang),
		IgnoreGlobs: ignoreGlobList,
	}

	result := analyzer.ComputeHotspots(graphData, opts)

	switch hotspotsFormat {
	case "json":
		if err := output.PrintJSON(result); err != nil {
			return fmt.Errorf("writing JSON: %w", err)
		}
	default:
		output.PrintHotspots(result)
	}

	return nil
}
