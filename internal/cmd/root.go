package cmd

import (
	"os"

	"github.com/anurag/tracescope/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	verbose bool
	cfg     config.Config
)

var rootCmd = &cobra.Command{
	Use:   "tracescope",
	Short: "TraceScope — dependency graph & blast radius analyzer",
	Long:  "TraceScope parses codebases into dependency graphs and analyzes PR blast radius.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		if verbose {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		}
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

		// Load project config
		cwd, _ := os.Getwd()
		loaded, path, err := config.Load(cwd)
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("failed to load config")
		} else {
			cfg = loaded
			if path != "" {
				log.Debug().Str("path", path).Msg("loaded config")
			}
		}
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}

func Execute() error {
	return rootCmd.Execute()
}
