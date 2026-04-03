package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/parser"
	scip "github.com/scip-code/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
)

func TestRunIndex_PrefersSCIPWhenPresent(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.scip")

	index := &scip.Index{
		Documents: []*scip.Document{{
			RelativePath: "main.go",
			Language:     "go",
			Occurrences: []*scip.Occurrence{{
				Range:       []int32{0, 5, 9},
				Symbol:      "scip-go gomod example.com/acme/cmd main().",
				SymbolRoles: int32(scip.SymbolRole_Definition),
			}},
			Symbols: []*scip.SymbolInformation{{
				Symbol:      "scip-go gomod example.com/acme/cmd main().",
				Kind:        scip.SymbolInformation_Function,
				DisplayName: "main",
			}},
		}},
	}

	raw, err := proto.Marshal(index)
	if err != nil {
		t.Fatalf("marshal scip fixture: %v", err)
	}
	if err := os.WriteFile(indexPath, raw, 0o600); err != nil {
		t.Fatalf("write index.scip: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	if err := runIndex(indexCmd, []string{dir}); err != nil {
		t.Fatalf("runIndex failed: %v", err)
	}

	gd, err := graph.NewStore().Load(filepath.Join(dir, ".tracescope", "graph.json"))
	if err != nil {
		t.Fatalf("load graph.json: %v", err)
	}
	if gd.IndexSource != "scip" {
		t.Fatalf("expected scip index source, got %q", gd.IndexSource)
	}
	assertIndexerStatus(t, gd.IndexerStatuses, "index.scip", "used_existing")
}

func TestRunIndex_FallsBackToParserWithoutSCIP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`
package main

func main() {}
`), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	if err := runIndex(indexCmd, []string{dir}); err != nil {
		t.Fatalf("runIndex failed: %v", err)
	}

	gd, err := graph.NewStore().Load(filepath.Join(dir, ".tracescope", "graph.json"))
	if err != nil {
		t.Fatalf("load graph.json: %v", err)
	}
	if gd.IndexSource != "parser" {
		t.Fatalf("expected parser index source, got %q", gd.IndexSource)
	}
	assertIndexerStatus(t, gd.IndexerStatuses, "scip-go", "skipped")
	if len(gd.Nodes) == 0 {
		t.Fatal("expected parser fallback graph nodes")
	}
}

func TestGenerateSCIPIndex_SelectsGoIndexer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/acme\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	var called []string
	resetSCIPHooks := stubSCIPHooks(t, map[string]bool{"scip-go": true}, func(workdir, name string, args ...string) error {
		called = append(called, fmt.Sprintf("%s:%s:%v", workdir, name, args))
		return os.WriteFile(filepath.Join(workdir, "index.scip"), []byte("stub"), 0o600)
	})
	defer resetSCIPHooks()

	generated, statuses, err := generateSCIPIndexes(dir, filepath.Join(dir, ".tracescope", "scip"), map[parser.Language][]string{
		parser.LangGo: {filepath.Join(dir, "main.go")},
	}, filepath.Join(dir, "index.scip"))
	if err != nil {
		t.Fatalf("generateSCIPIndexes failed: %v", err)
	}
	if len(generated) != 1 {
		t.Fatal("expected SCIP generation")
	}
	if called[0] != fmt.Sprintf("%s:scip-go:[]", dir) {
		t.Fatalf("expected scip-go invocation, got %q", called[0])
	}
	if filepath.Base(generated[0]) != "scip-go.scip" {
		t.Fatalf("expected scip-go.scip output, got %q", generated[0])
	}
	assertIndexerStatus(t, statuses, "scip-go", "generated")
}

func TestGenerateSCIPIndex_SelectsTypeScriptIndexer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	var called []string
	resetSCIPHooks := stubSCIPHooks(t, map[string]bool{"scip-typescript": true}, func(workdir, name string, args ...string) error {
		called = append(called, fmt.Sprintf("%s:%s:%v", workdir, name, args))
		return os.WriteFile(filepath.Join(workdir, "index.scip"), []byte("stub"), 0o600)
	})
	defer resetSCIPHooks()

	generated, statuses, err := generateSCIPIndexes(dir, filepath.Join(dir, ".tracescope", "scip"), map[parser.Language][]string{
		parser.LangTypeScript: {filepath.Join(dir, "app.ts")},
	}, filepath.Join(dir, "index.scip"))
	if err != nil {
		t.Fatalf("generateSCIPIndexes failed: %v", err)
	}
	if len(generated) != 1 {
		t.Fatal("expected SCIP generation")
	}
	if called[0] != fmt.Sprintf("%s:scip-typescript:[index]", dir) {
		t.Fatalf("expected scip-typescript index invocation, got %q", called[0])
	}
	if filepath.Base(generated[0]) != "scip-typescript.scip" {
		t.Fatalf("expected scip-typescript.scip output, got %q", generated[0])
	}
	assertIndexerStatus(t, statuses, "scip-typescript", "generated")
}

func TestGenerateSCIPIndex_SelectsPythonIndexer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scip-python is intentionally skipped on native Windows")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname='demo'\n"), 0o600); err != nil {
		t.Fatalf("write pyproject.toml: %v", err)
	}

	var called []string
	resetSCIPHooks := stubSCIPHooks(t, map[string]bool{"scip-python": true}, func(workdir, name string, args ...string) error {
		called = append(called, fmt.Sprintf("%s:%s:%v", workdir, name, args))
		return os.WriteFile(filepath.Join(workdir, "index.scip"), []byte("stub"), 0o600)
	})
	defer resetSCIPHooks()

	generated, statuses, err := generateSCIPIndexes(dir, filepath.Join(dir, ".tracescope", "scip"), map[parser.Language][]string{
		parser.LangPython: {filepath.Join(dir, "app.py")},
	}, filepath.Join(dir, "index.scip"))
	if err != nil {
		t.Fatalf("generateSCIPIndexes failed: %v", err)
	}
	if len(generated) != 1 {
		t.Fatal("expected SCIP generation")
	}
	if called[0] != fmt.Sprintf("%s:scip-python:[index . --project-name %s]", dir, filepath.Base(dir)) {
		t.Fatalf("expected scip-python invocation, got %q", called[0])
	}
	if filepath.Base(generated[0]) != "scip-python.scip" {
		t.Fatalf("expected scip-python.scip output, got %q", generated[0])
	}
	assertIndexerStatus(t, statuses, "scip-python", "generated")
}

func TestGenerateSCIPIndex_SkipsPythonIndexerOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only scip-python skip behavior")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname='demo'\n"), 0o600); err != nil {
		t.Fatalf("write pyproject.toml: %v", err)
	}

	resetSCIPHooks := stubSCIPHooks(t, map[string]bool{"scip-python": true}, func(string, string, ...string) error {
		t.Fatal("scip-python should be skipped on Windows")
		return nil
	})
	defer resetSCIPHooks()

	generated, statuses, err := generateSCIPIndexes(dir, filepath.Join(dir, ".tracescope", "scip"), map[parser.Language][]string{
		parser.LangPython: {filepath.Join(dir, "app.py")},
	}, filepath.Join(dir, "index.scip"))
	if err != nil {
		t.Fatalf("generateSCIPIndexes failed: %v", err)
	}
	if len(generated) != 0 {
		t.Fatalf("expected no generated SCIP files on Windows, got %v", generated)
	}
	assertIndexerStatus(t, statuses, "scip-python", "skipped")
}

func TestGenerateSCIPIndexes_MergesMultipleLanguageIndexes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/acme\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	var called []string
	resetSCIPHooks := stubSCIPHooks(t, map[string]bool{
		"scip-go":         true,
		"scip-typescript": true,
	}, func(workdir, name string, args ...string) error {
		called = append(called, name)
		return os.WriteFile(filepath.Join(workdir, "index.scip"), []byte(name), 0o600)
	})
	defer resetSCIPHooks()

	generated, statuses, err := generateSCIPIndexes(dir, filepath.Join(dir, ".tracescope", "scip"), map[parser.Language][]string{
		parser.LangGo:         {filepath.Join(dir, "main.go")},
		parser.LangTypeScript: {filepath.Join(dir, "app.ts")},
	}, filepath.Join(dir, "index.scip"))
	if err != nil {
		t.Fatalf("generateSCIPIndexes failed: %v", err)
	}
	if len(generated) != 2 {
		t.Fatalf("expected 2 generated SCIP files, got %d (%v)", len(generated), generated)
	}
	sort.Strings(called)
	if fmt.Sprint(called) != "[scip-go scip-typescript]" {
		t.Fatalf("expected scip-go and scip-typescript invocations, got %v", called)
	}
	assertIndexerStatus(t, statuses, "scip-go", "generated")
	assertIndexerStatus(t, statuses, "scip-typescript", "generated")
}

func TestGenerateSCIPIndexes_ReusesFreshCache(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.go")
	outputDir := filepath.Join(dir, ".tracescope", "scip")
	outputPath := filepath.Join(outputDir, "scip-go.scip")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/acme\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir scip output dir: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte("cached"), 0o600); err != nil {
		t.Fatalf("write cached index: %v", err)
	}
	now := time.Now()
	if err := os.Chtimes(sourcePath, now, now); err != nil {
		t.Fatalf("chtimes main.go: %v", err)
	}
	if err := os.Chtimes(filepath.Join(dir, "go.mod"), now, now); err != nil {
		t.Fatalf("chtimes go.mod: %v", err)
	}
	if err := os.Chtimes(outputPath, now.Add(time.Hour), now.Add(time.Hour)); err != nil {
		t.Fatalf("chtimes cached index: %v", err)
	}

	resetSCIPHooks := stubSCIPHooks(t, map[string]bool{"scip-go": true}, func(string, string, ...string) error {
		t.Fatal("runSCIPCommand should not be called for a fresh SCIP cache")
		return nil
	})
	defer resetSCIPHooks()

	generated, statuses, err := generateSCIPIndexes(dir, outputDir, map[parser.Language][]string{
		parser.LangGo: {sourcePath},
	}, filepath.Join(dir, "index.scip"))
	if err != nil {
		t.Fatalf("generateSCIPIndexes failed: %v", err)
	}
	if len(generated) != 1 || generated[0] != outputPath {
		t.Fatalf("expected cached SCIP output %q, got %v", outputPath, generated)
	}
	assertIndexerStatus(t, statuses, "scip-go", "cached")
}

func TestGenerateSCIPIndexes_RegeneratesStaleCache(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.go")
	outputDir := filepath.Join(dir, ".tracescope", "scip")
	outputPath := filepath.Join(outputDir, "scip-go.scip")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/acme\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir scip output dir: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale index: %v", err)
	}
	now := time.Now()
	if err := os.Chtimes(outputPath, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatalf("chtimes stale index: %v", err)
	}
	if err := os.Chtimes(sourcePath, now, now); err != nil {
		t.Fatalf("chtimes main.go: %v", err)
	}

	calls := 0
	resetSCIPHooks := stubSCIPHooks(t, map[string]bool{"scip-go": true}, func(workdir, name string, args ...string) error {
		calls++
		return os.WriteFile(filepath.Join(workdir, "index.scip"), []byte(name), 0o600)
	})
	defer resetSCIPHooks()

	generated, statuses, err := generateSCIPIndexes(dir, outputDir, map[parser.Language][]string{
		parser.LangGo: {sourcePath},
	}, filepath.Join(dir, "index.scip"))
	if err != nil {
		t.Fatalf("generateSCIPIndexes failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one SCIP regeneration, got %d", calls)
	}
	if len(generated) != 1 || generated[0] != outputPath {
		t.Fatalf("expected regenerated SCIP output %q, got %v", outputPath, generated)
	}
	assertIndexerStatus(t, statuses, "scip-go", "generated")
}

func TestGenerateSCIPIndex_FallsBackWhenIndexerMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/acme\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	resetSCIPHooks := stubSCIPHooks(t, map[string]bool{}, func(string, string, ...string) error {
		t.Fatal("runSCIPCommand should not be called when indexer is missing")
		return nil
	})
	defer resetSCIPHooks()

	generated, statuses, err := generateSCIPIndexes(dir, filepath.Join(dir, ".tracescope", "scip"), map[parser.Language][]string{
		parser.LangGo: {filepath.Join(dir, "main.go")},
	}, filepath.Join(dir, "index.scip"))
	if err != nil {
		t.Fatalf("generateSCIPIndexes failed: %v", err)
	}
	if len(generated) > 0 {
		t.Fatal("expected fallback when no SCIP indexer is installed")
	}
	assertIndexerStatus(t, statuses, "scip-go", "missing_binary")
}

func stubSCIPHooks(t *testing.T, installed map[string]bool, run func(string, string, ...string) error) func() {
	t.Helper()

	originalLookPath := scipLookPath
	originalRunCommand := runSCIPCommand

	scipLookPath = func(file string) (string, error) {
		if installed[file] {
			return file, nil
		}
		return "", fmt.Errorf("%s not found", file)
	}
	runSCIPCommand = run

	return func() {
		scipLookPath = originalLookPath
		runSCIPCommand = originalRunCommand
	}
}

func assertIndexerStatus(t *testing.T, statuses []graph.IndexerStatus, name, state string) {
	t.Helper()

	for _, status := range statuses {
		if status.Name == name {
			if status.State != state {
				t.Fatalf("expected %s state %q, got %q (%+v)", name, state, status.State, status)
			}
			return
		}
	}
	t.Fatalf("expected status for %s in %+v", name, statuses)
}
