package cmd

import (
	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/server"
	"github.com/spf13/cobra"
)

var servePort int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the TraceScope API server",
	Long: `Starts an HTTP server exposing the TraceScope engine as a REST + WebSocket API.
Used by the Next.js dashboard and other integrations.

Examples:
  tracescope serve              # default port 4000
  tracescope serve --port 8080`,
	RunE:          runServe,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 4000, "port to listen on")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	return server.ListenAndServe(server.Config{
		Port: servePort,
		Scorer: &analyzer.RiskScorer{
			HighCallers:         cfg.Risk.HighCallers,
			HighExportedCallers: cfg.Risk.HighExportedCallers,
			MediumCallers:       cfg.Risk.MediumCallers,
		},
	})
}
