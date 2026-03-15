package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Load from a directory with no config file
	dir := t.TempDir()
	cfg, path, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected no config path, got %q", path)
	}
	if cfg.MaxDepth != 5 {
		t.Errorf("expected default max_depth 5, got %d", cfg.MaxDepth)
	}
	if cfg.Format != "terminal" {
		t.Errorf("expected default format 'terminal', got %q", cfg.Format)
	}
	if cfg.Risk.HighCallers != 10 {
		t.Errorf("expected default high_callers 10, got %d", cfg.Risk.HighCallers)
	}
}

func TestLoad_FullConfig(t *testing.T) {
	dir := t.TempDir()
	content := `
ignore:
  - vendor/**
  - dist/**
max_depth: 10
format: json
top: 20
graph_path: custom/graph.json
risk:
  high_callers: 15
  high_exported_callers: 8
  medium_callers: 5
`
	if err := os.WriteFile(filepath.Join(dir, ".tracescope.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, path, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected config path")
	}
	if cfg.MaxDepth != 10 {
		t.Errorf("expected max_depth 10, got %d", cfg.MaxDepth)
	}
	if cfg.Format != "json" {
		t.Errorf("expected format 'json', got %q", cfg.Format)
	}
	if cfg.TopN != 20 {
		t.Errorf("expected top 20, got %d", cfg.TopN)
	}
	if len(cfg.Ignore) != 2 {
		t.Errorf("expected 2 ignore patterns, got %d", len(cfg.Ignore))
	}
	if cfg.Risk.HighCallers != 15 {
		t.Errorf("expected high_callers 15, got %d", cfg.Risk.HighCallers)
	}
	if cfg.Risk.HighExportedCallers != 8 {
		t.Errorf("expected high_exported_callers 8, got %d", cfg.Risk.HighExportedCallers)
	}
	if cfg.Risk.MediumCallers != 5 {
		t.Errorf("expected medium_callers 5, got %d", cfg.Risk.MediumCallers)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	content := `max_depth: 3`
	if err := os.WriteFile(filepath.Join(dir, ".tracescope.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxDepth != 3 {
		t.Errorf("expected max_depth 3, got %d", cfg.MaxDepth)
	}
	// Other fields should keep defaults
	if cfg.Format != "terminal" {
		t.Errorf("expected default format 'terminal', got %q", cfg.Format)
	}
	if cfg.Risk.HighCallers != 10 {
		t.Errorf("expected default high_callers 10, got %d", cfg.Risk.HighCallers)
	}
}

func TestLoad_WalkUp(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	content := `max_depth: 7`
	if err := os.WriteFile(filepath.Join(root, ".tracescope.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, path, err := Load(sub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected config file to be found by walking up")
	}
	if cfg.MaxDepth != 7 {
		t.Errorf("expected max_depth 7, got %d", cfg.MaxDepth)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tracescope.yaml"), []byte("{{invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tracescope.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should get all defaults
	if cfg.MaxDepth != 5 {
		t.Errorf("expected default max_depth 5, got %d", cfg.MaxDepth)
	}
}
