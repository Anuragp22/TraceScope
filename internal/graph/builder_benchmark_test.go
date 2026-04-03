package graph

import (
	"fmt"
	"testing"

	"github.com/anurag/tracescope/internal/parser"
)

func BenchmarkBuilder_LargeTSProject(b *testing.B) {
	results := makeBenchmarkTSProject(800)
	builder := NewBuilder()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gd := builder.Build(results)
		if len(gd.Nodes) == 0 {
			b.Fatal("expected graph nodes")
		}
	}
}

func BenchmarkComputeBlastRadius_LargeFanInGraph(b *testing.B) {
	results := makeBenchmarkTSProject(800)
	gd := NewBuilder().Build(results)

	seedID := ""
	for i := range gd.Nodes {
		if gd.Nodes[i].Type == NodeFunction && gd.Nodes[i].Name == "leaf0" {
			seedID = gd.Nodes[i].ID
			break
		}
	}
	if seedID == "" {
		b.Fatal("seed function not found")
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		br := ComputeBlastRadius(gd, []string{seedID}, 8)
		if len(br.AffectedNodes) == 0 {
			b.Fatal("expected affected nodes")
		}
	}
}

func makeBenchmarkTSProject(files int) []*parser.FileResult {
	results := make([]*parser.FileResult, 0, files+1)
	results = append(results, &parser.FileResult{
		FilePath: "src/shared/leaf.ts",
		Language: parser.LangTypeScript,
		Functions: []parser.FunctionDef{
			{Name: "leaf0", StartLine: 1, EndLine: 3, IsExport: true},
		},
	})

	for i := 0; i < files; i++ {
		results = append(results, &parser.FileResult{
			FilePath: fmt.Sprintf("src/pkg%03d/service.ts", i),
			Language: parser.LangTypeScript,
			Functions: []parser.FunctionDef{
				{Name: fmt.Sprintf("handler%03d", i), StartLine: 1, EndLine: 10, IsExport: i%3 == 0},
			},
			Imports: []parser.Import{
				{Path: "../shared/leaf", Alias: "leaf0", Symbol: "leaf0", Line: 1},
			},
			Calls: []parser.FunctionCall{
				{Name: "leaf0", Line: 4},
			},
		})
	}

	return results
}
