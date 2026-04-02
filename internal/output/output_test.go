package output

import (
	"strings"
	"testing"

	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
)

func TestFormatMarkdownComment(t *testing.T) {
	result := &analyzer.AnalysisResult{
		ChangedFiles: []diff.ChangedFile{
			{Path: "src/app.go"},
		},
		ChangedFunctions: []analyzer.ChangedFunction{
			{NodeID: "f1", Node: &graph.Node{Name: "main", FilePath: "src/app.go", StartLine: 10}},
		},
		AffectedFunctions: []analyzer.AffectedFunction{
			{
				Node:        &graph.Node{Name: "handleRequest", FilePath: "src/handler.go", StartLine: 25},
				Depth:       1,
				Risk:        analyzer.RiskHigh,
				Confidence:  graph.EdgeConfidenceExact,
				CallerCount: 5,
				Reason:      "exported function with many callers",
				ImpactPath: []graph.PathStep{
					{Node: &graph.Node{Name: "main"}},
					{Node: &graph.Node{Name: "handleRequest"}, EdgeType: "CALLS"},
				},
			},
			{
				Node:        &graph.Node{Name: "helper", FilePath: "src/util.go", StartLine: 10},
				Depth:       2,
				Risk:        analyzer.RiskLow,
				CallerCount: 1,
				Reason:      "internal function",
			},
		},
		TotalNodes: 100,
		TotalEdges: 200,
		MaxDepth:   5,
	}

	md := FormatMarkdownComment(result)

	// Check marker
	if !strings.Contains(md, commentMarker) {
		t.Error("missing comment marker")
	}

	// Check header
	if !strings.Contains(md, "## TraceScope") {
		t.Error("missing header")
	}

	// Check summary table
	if !strings.Contains(md, "| Changed files | 1 |") {
		t.Error("missing changed files count")
	}

	// Check high risk section
	if !strings.Contains(md, "High Risk") {
		t.Error("missing high risk section")
	}
	if !strings.Contains(md, "Reviewer Focus") {
		t.Error("missing reviewer focus section")
	}
	if !strings.Contains(md, "`handleRequest`") {
		t.Error("missing affected function name")
	}
	if !strings.Contains(md, "`main` -> `handleRequest`") {
		t.Error("missing impact path")
	}

	// Check low risk is in collapsible section
	if !strings.Contains(md, "<details>") {
		t.Error("low risk should be in collapsible section")
	}
}

func TestHighestRiskExitCode(t *testing.T) {
	tests := []struct {
		name  string
		funcs []analyzer.AffectedFunction
		want  int
	}{
		{"no affected", nil, 0},
		{"only low", []analyzer.AffectedFunction{{Risk: analyzer.RiskLow}}, 0},
		{"medium", []analyzer.AffectedFunction{{Risk: analyzer.RiskMedium}}, 2},
		{"high", []analyzer.AffectedFunction{{Risk: analyzer.RiskHigh}}, 1},
		{"high beats medium", []analyzer.AffectedFunction{
			{Risk: analyzer.RiskMedium},
			{Risk: analyzer.RiskHigh},
		}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &analyzer.AnalysisResult{AffectedFunctions: tt.funcs}
			got := analyzer.HighestRiskExitCode(result)
			if got != tt.want {
				t.Errorf("HighestRiskExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}
