package main

import (
	"errors"
	"os"

	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var riskErr *analyzer.RiskExitError
		if errors.As(err, &riskErr) {
			os.Exit(riskErr.Code)
		}
		os.Exit(1)
	}
}
