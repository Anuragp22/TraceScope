package main

import (
	"os"

	"github.com/anurag/tracescope/internal/cmd"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
