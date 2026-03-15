package cmd

import (
	"testing"

	diffpkg "github.com/anurag/tracescope/internal/diff"
)

func TestFilterIgnoredFiles(t *testing.T) {
	files := []diffpkg.ChangedFile{
		{Path: "vendor/foo/bar.go"},
		{Path: "internal/parser/golang.go"},
		{Path: "dist/bundle.js"},
		{Path: "src/app.ts"},
		{Path: "generated/types.go"},
	}

	// Test vendor/** pattern
	result := filterIgnoredFiles(files, "vendor/**")
	if len(result) != 4 {
		t.Errorf("expected 4 files after filtering vendor/**, got %d", len(result))
	}

	// Test multiple patterns
	result = filterIgnoredFiles(files, "vendor/**, dist/**")
	if len(result) != 3 {
		t.Errorf("expected 3 files after filtering vendor/** and dist/**, got %d", len(result))
	}

	// Test filename glob
	result = filterIgnoredFiles(files, "*.js")
	if len(result) != 4 {
		t.Errorf("expected 4 files after filtering *.js, got %d", len(result))
	}

	// Test no match
	result = filterIgnoredFiles(files, "nonexistent/**")
	if len(result) != 5 {
		t.Errorf("expected 5 files (no match), got %d", len(result))
	}
}

func TestMatchesAnyGlob(t *testing.T) {
	tests := []struct {
		path  string
		globs []string
		want  bool
	}{
		{"vendor/foo/bar.go", []string{"vendor/**"}, true},
		{"vendor/deep/nested/file.go", []string{"vendor/**"}, true},
		{"src/vendor/file.go", []string{"vendor/**"}, false},
		{"dist/bundle.js", []string{"dist/**"}, true},
		{"src/app.ts", []string{"*.ts"}, true},
		{"src/app.ts", []string{"*.go"}, false},
		{"file.go", []string{""}, false},
	}

	for _, tt := range tests {
		got := matchesAnyGlob(tt.path, tt.globs)
		if got != tt.want {
			t.Errorf("matchesAnyGlob(%q, %v) = %v, want %v", tt.path, tt.globs, got, tt.want)
		}
	}
}
