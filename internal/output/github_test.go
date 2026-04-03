package output

import (
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
