package parser

import (
	"os"
	"path/filepath"
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
