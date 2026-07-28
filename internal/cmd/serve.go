package cmd

import (
	"os"
	"path/filepath"

	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/server"
	"github.com/spf13/cobra"
)

var (
	servePort int
	serveHost string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the TraceScope API server",
	Long: `Starts an HTTP REST API exposing the TraceScope engine.
Used by the Next.js dashboard and other integrations.

The server binds to localhost by default because it has no authentication and
can run git against the working tree. Use --host 0.0.0.0 only on a trusted
network where you deliberately want remote access.

Examples:
  tracescope serve                 # localhost, port 4000
  tracescope serve --port 8080
  tracescope serve --host 0.0.0.0  # expose on all interfaces (unauthenticated)`,
	RunE:          runServe,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 4000, "port to listen on")
	serveCmd.Flags().StringVar(&serveHost, "host", "127.0.0.1", "host/interface to bind (default loopback only)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	graphFile := resolveServeGraphFile(cfg.GraphPath, cwd)

	return server.ListenAndServe(server.Config{
		Host:      serveHost,
		Port:      servePort,
		GraphFile: graphFile,
		Scorer: &analyzer.RiskScorer{
			HighCallers:         cfg.Risk.HighCallers,
			HighExportedCallers: cfg.Risk.HighExportedCallers,
			MediumCallers:       cfg.Risk.MediumCallers,
		},
	})
}

// resolveServeGraphFile honours graph_path from .tracescope.yaml the same way
// analyze, why and hotspots do via loadGraph. Without it, serve silently read
// the default .tracescope/graph.json and ignored a configured path. An empty
// result means "use the server's own default".
func resolveServeGraphFile(configuredPath, cwd string) string {
	if configuredPath == "" {
		return ""
	}
	if filepath.IsAbs(configuredPath) {
		return configuredPath
	}
	if cwd == "" {
		return configuredPath
	}
	return filepath.Join(cwd, configuredPath)
}
