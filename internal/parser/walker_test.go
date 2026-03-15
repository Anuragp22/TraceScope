package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkDirectory(t *testing.T) {
	// Create temp directory structure
	dir := t.TempDir()

	files := map[string]string{
		"main.go":                 "package main",
		"utils/helper.go":        "package utils",
		"src/app.js":             "const x = 1;",
		"src/app.ts":             "const x: number = 1;",
		"lib/module.py":          "def foo(): pass",
		"node_modules/pkg/a.js":  "skip",
		"vendor/dep/b.go":        "skip",
		".git/config":            "skip",
		"__pycache__/cache.py":   "skip",
	}

	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(content), 0644)
	}

	result, err := WalkDirectory(dir)
	if err != nil {
		t.Fatalf("WalkDirectory failed: %v", err)
	}

	// Should find Go, JS, TS, Python files but NOT ones in skipped dirs
	if len(result[LangGo]) != 2 {
		t.Errorf("expected 2 Go files, got %d", len(result[LangGo]))
	}
	if len(result[LangJavaScript]) != 1 {
		t.Errorf("expected 1 JS file, got %d", len(result[LangJavaScript]))
	}
	if len(result[LangTypeScript]) != 1 {
		t.Errorf("expected 1 TS file, got %d", len(result[LangTypeScript]))
	}
	if len(result[LangPython]) != 1 {
		t.Errorf("expected 1 Python file, got %d", len(result[LangPython]))
	}
}

func TestWalkDirectory_Empty(t *testing.T) {
	dir := t.TempDir()
	result, err := WalkDirectory(dir)
	if err != nil {
		t.Fatalf("WalkDirectory failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d languages", len(result))
	}
}
