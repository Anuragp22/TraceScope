package config

import (
	"os"
	"path/filepath"
	"strings"
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

// TestLoad_PresentButInvalidValuesAreReported pins that a key written in the
// file is distinguishable from one left out. The merge previously took only
// non-zero values, so `max_depth: 0` was discarded in silence and the user was
// told nothing about why their setting had no effect.
func TestLoad_PresentButInvalidValuesAreReported(t *testing.T) {
	dir := t.TempDir()
	body := "max_depth: 0\nformat: yaml\nrisk:\n  high_callers: 0\n"
	if err := os.WriteFile(filepath.Join(dir, ".tracescope.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, path, err := Load(dir)
	if path == "" {
		t.Fatal("expected the config file to be found")
	}
	if err == nil {
		t.Fatal("expected out-of-range values to be reported, got nil")
	}
	for _, want := range []string{"max_depth", "format", "risk.high_callers"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q named in the report, got: %v", want, err)
		}
	}

	// Invalid values must not take effect — the defaults stand.
	if cfg.MaxDepth != 5 {
		t.Errorf("max_depth = %d, want the default 5", cfg.MaxDepth)
	}
	if cfg.Format != "terminal" {
		t.Errorf("format = %q, want the default terminal", cfg.Format)
	}
	if cfg.Risk.HighCallers != 10 {
		t.Errorf("risk.high_callers = %d, want the default 10", cfg.Risk.HighCallers)
	}
}

// TestLoad_ZeroTopIsHonoured asserts the one key where zero is meaningful:
// top: 0 means "show everything", and must be accepted rather than treated as
// absent.
func TestLoad_ZeroTopIsHonoured(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tracescope.yaml"), []byte("top: 0\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, _, err := Load(dir)
	if err != nil {
		t.Fatalf("top: 0 should be valid, got: %v", err)
	}
	if cfg.TopN != 0 {
		t.Errorf("top = %d, want 0", cfg.TopN)
	}
}
