package analyzer

import (
	"fmt"
	"testing"

	"github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
)

func BenchmarkBlastRadiusAnalyzer_AnalyzeLargeFanInGraph(b *testing.B) {
	gd := makeBenchmarkGraphData(1200)
	changedFiles := []diff.ChangedFile{{
		Path:       "src/core/target.go",
		LineRanges: []diff.LineRange{{Start: 2, End: 2}},
	}}
	analyzer := NewBlastRadiusAnalyzer(gd, 6, nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result := analyzer.Analyze(changedFiles)
		if len(result.AffectedFunctions) == 0 {
			b.Fatal("expected affected functions")
		}
	}
}

func makeBenchmarkGraphData(callers int) *graph.GraphData {
	nodes := make([]graph.Node, 0, callers+2)
	edges := make([]graph.Edge, 0, callers+1)

	nodes = append(nodes,
		graph.Node{ID: "file:target", Type: graph.NodeFile, Name: "target.go", FilePath: "src/core/target.go"},
		graph.Node{ID: "func:target", Type: graph.NodeFunction, Name: "Target", FilePath: "src/core/target.go", StartLine: 1, EndLine: 4, IsExport: true},
	)
	edges = append(edges, graph.Edge{
		Source:     "file:target",
		Target:     "func:target",
		Type:       graph.EdgeContains,
		Confidence: graph.EdgeConfidenceExact,
	})

	for i := 0; i < callers; i++ {
		fileID := fmt.Sprintf("file:caller:%d", i)
		funcID := fmt.Sprintf("func:caller:%d", i)
		filePath := fmt.Sprintf("src/service/caller_%04d.go", i)
		nodes = append(nodes,
			graph.Node{ID: fileID, Type: graph.NodeFile, Name: fmt.Sprintf("caller_%04d.go", i), FilePath: filePath},
			graph.Node{
				ID:        funcID,
				Type:      graph.NodeFunction,
				Name:      fmt.Sprintf("Caller%04d", i),
				FilePath:  filePath,
				StartLine: 1,
				EndLine:   8,
				IsExport:  i%4 == 0,
				IsTest:    i%10 == 0,
			},
		)
		edges = append(edges,
			graph.Edge{Source: fileID, Target: funcID, Type: graph.EdgeContains, Confidence: graph.EdgeConfidenceExact},
			graph.Edge{Source: funcID, Target: "func:target", Type: graph.EdgeCalls, Confidence: graph.EdgeConfidenceExact},
		)
	}

	return &graph.GraphData{Nodes: nodes, Edges: edges}
}
