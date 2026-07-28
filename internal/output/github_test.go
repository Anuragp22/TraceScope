package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
)

func TestFormatMarkdownComment_IncludesReviewerFocusContext(t *testing.T) {
	result := &analyzer.AnalysisResult{
		ChangedFunctions: []analyzer.ChangedFunction{{
			NodeID:   "changed",
			FilePath: "internal/service/service.go",
			Node:     &graph.Node{ID: "changed", Name: "Run", FilePath: "internal/service/service.go", StartLine: 10},
		}},
		AffectedFunctions: []analyzer.AffectedFunction{{
			Node: &graph.Node{
				ID:        "affected",
				Name:      "Serve",
				FilePath:  "internal/server/server.go",
				StartLine: 42,
				IsExport:  true,
			},
			Depth:       1,
			Risk:        analyzer.RiskHigh,
			ReviewScore: 118,
			Confidence:  graph.EdgeConfidenceExact,
			CallerCount: 7,
			Reason:      "7 total callers, 7 production callers, exported API",
			LastAuthor:  "@alice",
			ImpactPath: []graph.PathStep{
				{Node: &graph.Node{ID: "changed", Name: "Run"}},
				{Node: &graph.Node{ID: "affected", Name: "Serve"}, EdgeType: string(graph.EdgeCalls)},
			},
		}},
		ChangedFiles: []diff.ChangedFile{{Path: "internal/service/service.go"}},
		TotalNodes:   20,
		TotalEdges:   30,
		MaxDepth:     5,
	}

	comment := FormatMarkdownComment(result)
	for _, expected := range []string{
		"### Reviewer Focus",
		"| Score | Risk | Function | File | Owner | Confidence | Why | Impact path | Inspect first |",
		"| 118 | HIGH | `Serve` | `internal/server/server.go:42` | @alice | exact | 7 total callers, 7 production callers, exported API | `Run` -> `Serve` | Check API contract and direct callers first. |",
		"Prioritize high-score public/runtime paths first",
	} {
		if !strings.Contains(comment, expected) {
			t.Fatalf("expected comment to contain %q, got:\n%s", expected, comment)
		}
	}
}

// TestFormatMarkdownComment_DoesNotLeakAbsolutePaths pins that graph paths are
// rendered relative to the working directory. Nodes store absolute paths from
// whichever machine ran `tracescope index`, and this comment is posted publicly
// — printing them raw published that machine's directory layout, e.g.
// C:\Users\anura\Downloads\Devlopment\TraceScope\internal\analyzer\...
func TestFormatMarkdownComment_DoesNotLeakAbsolutePaths(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	abs := filepath.Join(cwd, "internal", "analyzer", "blast_radius.go")

	result := &analyzer.AnalysisResult{
		AffectedFunctions: []analyzer.AffectedFunction{{
			Node:        &graph.Node{ID: "a", Name: "Analyze", FilePath: abs, StartLine: 95, IsExport: true},
			Depth:       1,
			Risk:        analyzer.RiskHigh,
			ReviewScore: 112,
			Confidence:  graph.EdgeConfidenceExact,
			Reason:      "exported function with 5 production callers",
		}},
		ResolutionIssues: []graph.ResolutionIssue{{
			Kind: "call", Status: "unresolved", FilePath: abs, Line: 12, Symbol: "Foo",
		}},
		MaxDepth: 5,
	}

	comment := FormatMarkdownComment(result)

	if strings.Contains(comment, cwd) {
		t.Errorf("comment leaks the absolute working directory %q", cwd)
	}
	if strings.Contains(comment, "\\") {
		t.Errorf("comment contains Windows path separators; paths must be slash-normalised")
	}
	if !strings.Contains(comment, "internal/analyzer/blast_radius.go:95") {
		t.Errorf("expected a repo-relative path in the risk table, got:\n%s", comment)
	}
	if !strings.Contains(comment, "internal/analyzer/blast_radius.go:12") {
		t.Errorf("expected a repo-relative path in the diagnostics table, got:\n%s", comment)
	}
}

// TestRepoRelativePath_KeepsPathsOutsideTheBase asserts a path that is not
// under the base is left absolute rather than turned into a ../../ fragment
// that looks repo-relative but is not.
func TestRepoRelativePath_KeepsPathsOutsideTheBase(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(t.TempDir(), "other", "file.go")

	got := repoRelativePath(outside, base)
	if strings.HasPrefix(got, "..") {
		t.Errorf("path outside the base became a relative fragment %q", got)
	}
	if got != filepath.ToSlash(outside) {
		t.Errorf("repoRelativePath(%q, %q) = %q, want the slash-normalised original", outside, base, got)
	}

	// A relative path is already fine and must pass through untouched.
	if got := repoRelativePath("internal/x.go", base); got != "internal/x.go" {
		t.Errorf("relative input changed: %q", got)
	}
}
