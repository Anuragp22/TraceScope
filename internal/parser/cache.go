package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ParseCache stores FileResult objects keyed by file path for incremental indexing.
type ParseCache struct {
	Results map[string]*FileResult `json:"results"`
}

// NewParseCache creates an empty parse cache.
func NewParseCache() *ParseCache {
	return &ParseCache{Results: make(map[string]*FileResult)}
}

// SaveCache writes the parse cache to disk atomically.
//
// The temp file gets a unique name rather than a fixed path+".tmp": two indexes
// running against the same tree would otherwise write the same temp file and
// one would rename a half-written cache into place. graph.Store.Save already
// works this way.
func SaveCache(cache *ParseCache, path string) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".parse_cache-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
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
