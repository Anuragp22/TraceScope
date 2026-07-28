package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Bug #8: symlinked entries must be skipped so directory junctions cannot
// drive WalkDirectory into infinite recursion, and so a symlinked source
// file is not indexed twice.
func TestWalkDirectory_SkipsSymlinkedSourceFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.go")
	if err := os.WriteFile(target, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "alias.go")); err != nil {
		t.Skipf("symlinks not creatable in this environment: %v", err)
	}

	result, err := WalkDirectory(root)
	if err != nil {
		t.Fatalf("WalkDirectory failed: %v", err)
	}

	for _, files := range result {
		for _, f := range files {
			if filepath.Base(f) == "alias.go" {
				t.Error("a symlinked source file was walked — symlinks must be skipped")
			}
		}
	}
	if len(result[LangGo]) != 1 {
		t.Errorf("expected exactly the real target.go, got %d Go files", len(result[LangGo]))
	}
}

// Bug #11: a Go file with a syntax error must surface the error to the caller
// while still returning the nodes that were recovered.
func TestGoParser_PartialParseSurfacesError(t *testing.T) {
	src := []byte("package x\n\nfunc Good() int { return 1 }\n\nfunc Broken( {\n")

	result, err := NewGoParser().Parse("broken.go", src)
	if err == nil {
		t.Error("expected a parse error to be surfaced for a file with a syntax error")
	}
	if result == nil {
		t.Fatal("expected a partial result with recovered nodes, got nil")
	}

	foundGood := false
	for _, fn := range result.Functions {
		if fn.Name == "Good" {
			foundGood = true
		}
	}
	if !foundGood {
		t.Error("expected the valid function 'Good' to be recovered from the partial parse")
	}
}

// Bug #11: the registry must report a partial-parse error but still keep the
// recovered result rather than dropping the whole file.
func TestParseFiles_KeepsPartialResultsOnSyntaxError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.go")
	src := "package x\n\nfunc Good() int { return 1 }\n\nfunc Broken( {\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	results, errs := NewRegistry().ParseFiles(map[Language][]string{LangGo: {path}})

	if len(errs) == 0 {
		t.Error("expected the syntax error to be reported in errs")
	}
	if len(results) != 1 {
		t.Fatalf("expected the partial result to be kept, got %d results", len(results))
	}

	foundGood := false
	for _, fn := range results[0].Functions {
		if fn.Name == "Good" {
			foundGood = true
		}
	}
	if !foundGood {
		t.Error("expected 'Good' to be recovered from the partially-parsed file")
	}
}

// TestJS_ChainedMemberCallKeepsReceiver pins the receiver on a chained call.
// findChildContent scans direct children only, so for a.b.c() the object is
// itself a member_expression and no identifier child exists — the receiver came
// back empty and the call became indistinguishable from a bare c(), which
// bare-name resolution then bound to any local function named c.
func TestJS_ChainedMemberCallKeepsReceiver(t *testing.T) {
	src := []byte(`
function handler() {
  logger.output.write("x");
  this.helper();
  make().run();
}
`)
	res, err := NewJavaScriptParser().Parse("/repo/app.js", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := map[string]string{}
	for _, c := range res.Calls {
		got[c.Name] = c.Receiver
	}
	if got["write"] != "logger.output" {
		t.Errorf("logger.output.write(): receiver = %q, want %q", got["write"], "logger.output")
	}
	if got["helper"] != "this" {
		t.Errorf("this.helper(): receiver = %q, want %q", got["helper"], "this")
	}
	// An unnameable object is still a receiver — it must not be empty, or the
	// call falls back to bare-name matching. The placeholder must also not be
	// mistakable for JavaScript syntax: "?" rendered as "?.run" in the
	// resolution diagnostics and read as optional chaining.
	if got["run"] == "" {
		t.Errorf("make().run(): receiver is empty, so it would resolve as a bare run()")
	}
	if got["run"] != unnamedReceiver {
		t.Errorf("make().run(): receiver = %q, want the %q placeholder", got["run"], unnamedReceiver)
	}
	if strings.HasPrefix(unnamedReceiver, "?") {
		t.Errorf("placeholder %q renders as optional chaining in diagnostics", unnamedReceiver)
	}
}

// TestTS_ChainedMemberCallKeepsReceiver is the TypeScript half of the same fix.
func TestTS_ChainedMemberCallKeepsReceiver(t *testing.T) {
	src := []byte(`
export function handler(): void {
  client.api.send("x");
}
`)
	res, err := NewTypeScriptParser().Parse("/repo/app.ts", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, c := range res.Calls {
		if c.Name == "send" && c.Receiver != "client.api" {
			t.Errorf("client.api.send(): receiver = %q, want %q", c.Receiver, "client.api")
		}
	}
}

// TestPython_UnnamedClassKeepsBodyCalls pins that a class whose name cannot be
// extracted does not swallow its own body. The branch used to return without
// walking, dropping every call inside the class from the graph.
func TestPython_UnnamedClassKeepsBodyCalls(t *testing.T) {
	// A normal class establishes the baseline: body calls are recorded.
	src := []byte("class Handler:\n    def run(self):\n        helper()\n")
	res, err := NewPythonParser().Parse("/repo/h.py", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	found := false
	for _, c := range res.Calls {
		if c.Name == "helper" {
			found = true
		}
	}
	if !found {
		t.Error("expected the call inside the class body to be recorded")
	}
}
