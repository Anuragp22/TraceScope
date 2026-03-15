package graph

import (
	"encoding/json"
	"fmt"
	"os"
)

// Store handles persistence of graph data.
type Store struct{}

// NewStore creates a new graph store.
func NewStore() *Store {
	return &Store{}
}

// Save writes graph data to a JSON file.
func (s *Store) Save(data *GraphData, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("encoding graph: %w", err)
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
