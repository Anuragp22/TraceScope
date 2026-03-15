package parser

import (
	"encoding/json"
	"os"
)

// ParseCache stores FileResult objects keyed by file path for incremental indexing.
type ParseCache struct {
	Results map[string]*FileResult `json:"results"`
}

// NewParseCache creates an empty parse cache.
func NewParseCache() *ParseCache {
	return &ParseCache{Results: make(map[string]*FileResult)}
}

// SaveCache writes the parse cache to disk.
func SaveCache(cache *ParseCache, path string) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// LoadCache reads the parse cache from disk. Returns empty cache if file doesn't exist.
func LoadCache(path string) (*ParseCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewParseCache(), nil
		}
		return nil, err
	}
	cache := NewParseCache()
	if err := json.Unmarshal(data, cache); err != nil {
		return NewParseCache(), nil // corrupted cache — start fresh
	}
	return cache, nil
}
