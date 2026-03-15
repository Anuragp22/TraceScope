package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store handles persistence of graph data.
type Store struct{}

// NewStore creates a new graph store.
func NewStore() *Store {
	return &Store{}
}

// Save writes graph data to a JSON file atomically.
// It writes to a temporary file first, then renames to the final path.
func (s *Store) Save(data *GraphData, path string) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".graph-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	enc := json.NewEncoder(tmpFile)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encoding graph: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// Load reads graph data from a JSON file.
func (s *Store) Load(path string) (*GraphData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	var data GraphData
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding graph: %w", err)
	}

	return &data, nil
}
