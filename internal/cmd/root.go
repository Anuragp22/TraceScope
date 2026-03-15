package cmd

import (
	"github.com/spf13/cobra"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:   "tracescope",
	Short: "TraceScope — dependency graph & blast radius analyzer",
	Long:  "TraceScope parses codebases into dependency graphs and analyzes PR blast radius.",
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}

func Execute() error {
	return rootCmd.Execute()
}
