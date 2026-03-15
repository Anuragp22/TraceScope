package ownership

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCodeowners_Parse(t *testing.T) {
	dir := t.TempDir()
	content := `# This is a comment
*.go @backend-team
*.js @frontend-team @alice
/docs/ @docs-team
src/api/** @api-team
`
	ghDir := filepath.Join(dir, ".github")
	os.MkdirAll(ghDir, 0755)
	os.WriteFile(filepath.Join(ghDir, "CODEOWNERS"), []byte(content), 0644)

	co, err := LoadCodeowners(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if co == nil {
		t.Fatal("expected codeowners to be loaded")
	}
	if len(co.Rules) != 4 {
		t.Errorf("expected 4 rules, got %d", len(co.Rules))
	}
}

func TestCodeowners_Match(t *testing.T) {
	co := &Codeowners{
		Rules: []CodeownersRule{
			{Pattern: "*.go", Owners: []string{"@backend"}},
			{Pattern: "*.js", Owners: []string{"@frontend", "@alice"}},
			{Pattern: "src/api/**", Owners: []string{"@api-team"}},
		},
	}

	tests := []struct {
		path    string
		wantLen int
	}{
		{"internal/parser/golang.go", 1},
		{"app.js", 2},
		{"src/api/handler.go", 1},
		{"README.md", 0},
	}

	for _, tt := range tests {
		owners := co.Match(tt.path)
		if len(owners) != tt.wantLen {
			t.Errorf("Match(%q) = %v (len %d), want len %d", tt.path, owners, len(owners), tt.wantLen)
		}
	}
}

func TestCodeowners_LastMatchWins(t *testing.T) {
	co := &Codeowners{
		Rules: []CodeownersRule{
			{Pattern: "*.go", Owners: []string{"@general"}},
			{Pattern: "internal/**", Owners: []string{"@internal-team"}},
		},
	}

	owners := co.Match("internal/foo.go")
	if len(owners) != 1 || owners[0] != "@internal-team" {
		t.Errorf("expected @internal-team (last match wins), got %v", owners)
	}
}

func TestCodeowners_SuggestReviewers(t *testing.T) {
	co := &Codeowners{
		Rules: []CodeownersRule{
			{Pattern: "*.go", Owners: []string{"@alice"}},
			{Pattern: "*.js", Owners: []string{"@bob"}},
		},
	}

	reviewers := co.SuggestReviewers([]string{"a.go", "b.go", "c.go", "app.js"})

	if len(reviewers) != 2 {
		t.Fatalf("expected 2 reviewers, got %d", len(reviewers))
	}
	if reviewers[0].Owner != "@alice" || reviewers[0].FileCount != 3 {
		t.Errorf("expected @alice with 3 files first, got %s with %d", reviewers[0].Owner, reviewers[0].FileCount)
	}
}

func TestCodeowners_NilSafe(t *testing.T) {
	var co *Codeowners
	if co.Match("foo.go") != nil {
		t.Error("expected nil from nil Codeowners")
	}
	if co.SuggestReviewers([]string{"foo.go"}) != nil {
		t.Error("expected nil from nil Codeowners")
	}
}

func TestCodeowners_NotFound(t *testing.T) {
	dir := t.TempDir()
	co, err := LoadCodeowners(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if co != nil {
		t.Error("expected nil when no CODEOWNERS file")
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()
	tests := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-2 * time.Hour), "2 hours ago"},
		{now.Add(-48 * time.Hour), "2 days ago"},
		{now.Add(-60 * 24 * time.Hour), "2 months ago"},
		{time.Time{}, "unknown"},
	}
	for _, tt := range tests {
		got := RelativeTime(tt.t)
		if got != tt.want {
			t.Errorf("RelativeTime(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}
