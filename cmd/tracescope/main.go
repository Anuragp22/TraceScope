package main

import (
	"fmt"
	"os"

	"github.com/anurag/tracescope/internal/cmd"
)

func main() {
	code, message := cmd.ResolveExit(cmd.Execute())
	if message != "" {
		fmt.Fprintln(os.Stderr, message)
	}
	os.Exit(code)
}
