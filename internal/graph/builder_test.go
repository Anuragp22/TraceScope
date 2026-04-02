package graph

import (
	"testing"

	"github.com/anurag/tracescope/internal/parser"
)

func TestBuilder_BasicGraph(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "main.go",
			Language: parser.LangGo,
			Package:  "main",
			Functions: []parser.FunctionDef{
				{Name: "main", StartLine: 5, EndLine: 10, IsExport: false},
			},
			Calls: []parser.FunctionCall{
				{Name: "Run", Receiver: "", Line: 7},
			},
		},
		{
			FilePath: "server.go",
			Language: parser.LangGo,
			Package:  "main",
			Functions: []parser.FunctionDef{
				{Name: "Run", StartLine: 3, EndLine: 15, IsExport: true},
			},
		},
	}

	b := NewBuilder()
	gd := b.Build(results)

	// Should have 2 file nodes + 2 function nodes = 4
	if len(gd.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(gd.Nodes))
	}

	// Should have CONTAINS edges (2) + CALLS edge (1) = at least 3
	containsCount := 0
	callsCount := 0
	for _, e := range gd.Edges {
		switch e.Type {
		case EdgeContains:
			containsCount++
		case EdgeCalls:
			callsCount++
		}
	}
	if containsCount != 2 {
		t.Errorf("expected 2 CONTAINS edges, got %d", containsCount)
	}
	if callsCount != 1 {
		t.Errorf("expected 1 CALLS edge, got %d", callsCount)
	}
}

func TestBuilder_ClassNodes(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "models.go",
			Language: parser.LangGo,
			Package:  "models",
			Classes: []parser.ClassDef{
				{Name: "User", StartLine: 3, EndLine: 8, Kind: "struct", IsExport: true},
				{Name: "Service", StartLine: 10, EndLine: 20, Kind: "interface", IsExport: true},
			},
		},
	}

	b := NewBuilder()
	gd := b.Build(results)

	classCount := 0
	for _, n := range gd.Nodes {
		if n.Type == NodeClass {
			classCount++
		}
	}
	if classCount != 2 {
		t.Errorf("expected 2 class nodes, got %d", classCount)
	}
}

func TestBuilder_SkipsAmbiguousUnqualifiedCalls(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "caller/main.go",
			Language: parser.LangGo,
			Package:  "caller",
			Functions: []parser.FunctionDef{
				{Name: "main", StartLine: 1, EndLine: 5},
			},
			Calls: []parser.FunctionCall{
				{Name: "Run", Line: 2},
			},
		},
		{
			FilePath: "pkg/a/service.go",
			Language: parser.LangGo,
			Package:  "a",
			Functions: []parser.FunctionDef{
				{Name: "Run", StartLine: 1, EndLine: 3, IsExport: true},
			},
		},
		{
			FilePath: "pkg/b/service.go",
			Language: parser.LangGo,
			Package:  "b",
			Functions: []parser.FunctionDef{
				{Name: "Run", StartLine: 1, EndLine: 3, IsExport: true},
			},
		},
	}

	gd := NewBuilder().Build(results)

	for _, e := range gd.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		src := findNode(gd, e.Source)
		tgt := findNode(gd, e.Target)
		if src != nil && tgt != nil && src.Name == "main" && tgt.Name == "Run" {
			t.Fatal("expected ambiguous unqualified Run() call to remain unresolved")
		}
	}
}

func TestBuilder_ResolvesJSImportByFullFileQualifier(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "src/app/main.js",
			Language: parser.LangJavaScript,
			Functions: []parser.FunctionDef{
				{Name: "main", StartLine: 1, EndLine: 5},
			},
			Imports: []parser.Import{
				{Path: "../lib/service", Alias: "service", Line: 1},
			},
			Calls: []parser.FunctionCall{
				{Name: "start", Receiver: "service", Line: 3},
			},
		},
		{
			FilePath: "src/lib/service.js",
			Language: parser.LangJavaScript,
			Functions: []parser.FunctionDef{
				{Name: "start", StartLine: 1, EndLine: 3, IsExport: true},
			},
		},
		{
			FilePath: "src/other/service.js",
			Language: parser.LangJavaScript,
			Functions: []parser.FunctionDef{
				{Name: "start", StartLine: 1, EndLine: 3, IsExport: true},
			},
		},
	}

	gd := NewBuilder().Build(results)

	foundExpected := false
	for _, e := range gd.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		src := findNode(gd, e.Source)
		tgt := findNode(gd, e.Target)
		if src != nil && tgt != nil && src.FilePath == "src/app/main.js" && tgt.FilePath == "src/lib/service.js" {
			foundExpected = true
		}
		if src != nil && tgt != nil && src.FilePath == "src/app/main.js" && tgt.FilePath == "src/other/service.js" {
			t.Fatal("resolved imported service.start() to the wrong duplicate basename target")
		}
	}
	if !foundExpected {
		t.Fatal("expected service.start() to resolve to src/lib/service.js")
	}
}

func TestBuilder_ResolvesAliasedNamedJSImport(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "src/app/main.ts",
			Language: parser.LangTypeScript,
			Functions: []parser.FunctionDef{
				{Name: "main", StartLine: 1, EndLine: 6},
			},
			Imports: []parser.Import{
				{Path: "../lib/worker", Alias: "makeBuild", Symbol: "build", Line: 1},
			},
			Calls: []parser.FunctionCall{
				{Name: "makeBuild", Line: 3},
			},
		},
		{
			FilePath: "src/lib/worker.ts",
			Language: parser.LangTypeScript,
			Functions: []parser.FunctionDef{
				{Name: "build", StartLine: 1, EndLine: 3, IsExport: true},
			},
		},
		{
			FilePath: "src/other/worker.ts",
			Language: parser.LangTypeScript,
			Functions: []parser.FunctionDef{
				{Name: "build", StartLine: 1, EndLine: 3, IsExport: true},
			},
		},
	}

	gd := NewBuilder().Build(results)

	for _, e := range gd.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		src := findNode(gd, e.Source)
		tgt := findNode(gd, e.Target)
		if src != nil && tgt != nil && src.FilePath == "src/app/main.ts" && tgt.FilePath == "src/lib/worker.ts" {
			if e.Confidence != EdgeConfidenceExact {
				t.Fatalf("expected exact confidence for aliased named import, got %q", e.Confidence)
			}
			return
		}
		if src != nil && tgt != nil && src.FilePath == "src/app/main.ts" && tgt.FilePath == "src/other/worker.ts" {
			t.Fatal("resolved aliased named import to the wrong duplicate worker.ts target")
		}
	}

	t.Fatal("expected makeBuild() to resolve to src/lib/worker.ts build()")
}

func TestBuilder_ResolvesDefaultJSImport(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "src/app/main.js",
			Language: parser.LangJavaScript,
			Functions: []parser.FunctionDef{
				{Name: "main", StartLine: 1, EndLine: 6},
			},
			Imports: []parser.Import{
				{Path: "../lib/service", Alias: "runService", Symbol: "default", Line: 1},
			},
			Calls: []parser.FunctionCall{
				{Name: "runService", Line: 3},
			},
		},
		{
			FilePath: "src/lib/service.js",
			Language: parser.LangJavaScript,
			Functions: []parser.FunctionDef{
				{Name: "default", StartLine: 1, EndLine: 3, IsExport: true},
				{Name: "start", StartLine: 1, EndLine: 3, IsExport: true},
			},
		},
		{
			FilePath: "src/other/service.js",
			Language: parser.LangJavaScript,
			Functions: []parser.FunctionDef{
				{Name: "default", StartLine: 1, EndLine: 3, IsExport: true},
			},
		},
	}

	gd := NewBuilder().Build(results)

	for _, e := range gd.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		src := findNode(gd, e.Source)
		tgt := findNode(gd, e.Target)
		if src != nil && tgt != nil && src.FilePath == "src/app/main.js" && tgt.FilePath == "src/lib/service.js" && tgt.Name == "default" {
			return
		}
		if src != nil && tgt != nil && src.FilePath == "src/app/main.js" && tgt.FilePath == "src/other/service.js" {
			t.Fatal("resolved default import to the wrong duplicate service.js target")
		}
	}

	t.Fatal("expected runService() to resolve to src/lib/service.js default export")
}

func TestBuilder_ResolvesTypedTSMethodReceiver(t *testing.T) {
	source := []byte(`
class User {
  run() {
    return "user";
  }
}

class Service {
  run() {
    return "service";
  }
}

function main() {
  const user: User = new User();
  user.run();
}
`)

	fr, err := parser.NewTypeScriptParser().Parse("app.ts", source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	gd := NewBuilder().Build([]*parser.FileResult{fr})

	foundUserRun := false
	for _, e := range gd.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		src := findNode(gd, e.Source)
		tgt := findNode(gd, e.Target)
		if src != nil && tgt != nil && src.Name == "main" && tgt.Name == "run" {
			if tgt.ID != makeFuncID("app.ts", "run", "User", 3) {
				t.Fatalf("expected user.run() to resolve to User.run, got %s", tgt.ID)
			}
			if e.Confidence != EdgeConfidenceExact {
				t.Fatalf("expected exact confidence for typed TS receiver call, got %q", e.Confidence)
			}
			foundUserRun = true
		}
	}
	if !foundUserRun {
		t.Fatal("expected main -> User.run call edge")
	}
}

func TestBuilder_ResolvesGoImportsByPackageSuffix(t *testing.T) {
	results := []*parser.FileResult{
		{
			FilePath: "cmd/app/main.go",
			Language: parser.LangGo,
			Package:  "main",
			Functions: []parser.FunctionDef{
				{Name: "main", StartLine: 1, EndLine: 5},
			},
			Imports: []parser.Import{
				{Path: "github.com/acme/project/internal/service", Line: 1},
			},
			Calls: []parser.FunctionCall{
				{Name: "Run", Receiver: "service", Line: 3},
			},
		},
		{
			FilePath: "internal/service/run.go",
			Language: parser.LangGo,
			Package:  "service",
			Functions: []parser.FunctionDef{
				{Name: "Run", StartLine: 1, EndLine: 3, IsExport: true},
			},
		},
		{
			FilePath: "legacy/service/run.go",
			Language: parser.LangGo,
			Package:  "service",
			Functions: []parser.FunctionDef{
				{Name: "Run", StartLine: 1, EndLine: 3, IsExport: true},
			},
		},
	}

	gd := NewBuilder().Build(results)

	for _, e := range gd.Edges {
		if e.Type != EdgeImports {
			continue
		}
		src := findNode(gd, e.Source)
		tgt := findNode(gd, e.Target)
		if src != nil && tgt != nil && src.FilePath == "cmd/app/main.go" && tgt.FilePath == "internal/service/run.go" {
			return
		}
		if src != nil && tgt != nil && src.FilePath == "cmd/app/main.go" && tgt.FilePath == "legacy/service/run.go" {
			t.Fatal("resolved github.com/acme/project/internal/service to the wrong overlapping package suffix")
		}
	}

	t.Fatal("expected import to resolve to internal/service/run.go")
}

func TestBuilder_ResolvesTypedGoMethodReceiver(t *testing.T) {
	source := []byte(`package models

type User struct{}
type Service struct{}

func (User) Run() {}
func (Service) Run() {}

func main() {
	var user User
	user.Run()
}
`)
	fr, err := parser.NewGoParser().Parse("models.go", source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	gd := NewBuilder().Build([]*parser.FileResult{fr})

	foundUserRun := false
	for _, e := range gd.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		src := findNode(gd, e.Source)
		tgt := findNode(gd, e.Target)
		if src != nil && tgt != nil && src.Name == "main" && tgt.Name == "Run" {
			if tgt.ID != makeFuncID("models.go", "Run", "User", 6) {
				t.Fatalf("expected user.Run() to resolve to User.Run, got %s", tgt.ID)
			}
			if e.Confidence != EdgeConfidenceExact {
				t.Fatalf("expected exact confidence for typed receiver call, got %q", e.Confidence)
			}
			foundUserRun = true
		}
	}
	if !foundUserRun {
		t.Fatal("expected main -> User.Run call edge")
	}
}
