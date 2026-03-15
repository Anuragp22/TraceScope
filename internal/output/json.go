package output

import (
	"encoding/json"
	"os"
)

// PrintJSON outputs any value as indented JSON to stdout.
func PrintJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
