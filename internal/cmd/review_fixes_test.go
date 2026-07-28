package cmd

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/graph"
)

// Bug #3: a TraceScope error must produce a visible message and an exit code
// that does not collide with the risk gate (0/1/2).
func TestResolveExit(t *testing.T) {
	if code, msg := ResolveExit(nil); code != 0 || msg != "" {
		t.Errorf("nil error: got (%d, %q), want (0, \"\")", code, msg)
	}
	if code, msg := ResolveExit(&analyzer.RiskExitError{Code: 1}); code != 1 || msg != "" {
		t.Errorf("high-risk: got (%d, %q), want (1, \"\")", code, msg)
	}
	if code, msg := ResolveExit(&analyzer.RiskExitError{Code: 2}); code != 2 || msg != "" {
		t.Errorf("medium-risk: got (%d, %q), want (2, \"\")", code, msg)
	}

	code, msg := ResolveExit(errors.New("empty diff"))
	if code != 3 {
		t.Errorf("a TraceScope error must exit 3 (not collide with risk codes 0/1/2), got %d", code)
	}
	if !strings.Contains(msg, "empty diff") {
		t.Errorf("a TraceScope error must surface a visible message, got %q", msg)
	}
}

// Bug #6: a qualified name that matches more than one package must be reported
// as ambiguous, not silently resolved to the first candidate.
func TestResolveFunction_AmbiguousQualifiedMatch(t *testing.T) {
	gd := &graph.GraphData{
		Nodes: []graph.Node{
			{ID: "a", Type: graph.NodeFunction, Name: "Build", Package: "github.com/x/pkg", FilePath: "a.go"},
			{ID: "b", Type: graph.NodeFunction, Name: "Build", Package: "pkg", FilePath: "b.go"},
		},
	}

	if _, err := resolveFunction(gd, "pkg.Build"); err == nil {
		t.Fatal("expected an ambiguity error when a qualified name matches multiple packages")
	}
}

// TestResolveServeGraphFile pins that `serve` honours graph_path from
// .tracescope.yaml. It previously passed no GraphFile at all, so the server
// fell back to ./.tracescope/graph.json and silently ignored the configured
// path that every other command respects.
func TestResolveServeGraphFile(t *testing.T) {
	// t.TempDir gives a genuinely absolute path on every platform — a leading
	// slash is not absolute on Windows, which needs a volume name.
	cwd := t.TempDir()

	if got := resolveServeGraphFile("", cwd); got != "" {
		t.Errorf("no configured path should defer to the server default, got %q", got)
	}

	want := filepath.Join(cwd, "out", "graph.json")
	if got := resolveServeGraphFile(filepath.Join("out", "graph.json"), cwd); got != want {
		t.Errorf("relative graph_path = %q, want %q", got, want)
	}

	abs := filepath.Join(t.TempDir(), "graph.json")
	if got := resolveServeGraphFile(abs, cwd); got != abs {
		t.Errorf("absolute graph_path should pass through, got %q want %q", got, abs)
	}
}
