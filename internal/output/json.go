package output

import (
	"encoding/json"
	"os"

	"github.com/anurag/tracescope/internal/analyzer"
)

// PrintJSON outputs the analysis result as JSON to stdout.
func PrintJSON(result *analyzer.AnalysisResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
