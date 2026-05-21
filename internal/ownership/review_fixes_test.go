package ownership

import "testing"

// Bug #9: an anchored CODEOWNERS directory pattern (/dir/) must own every
// file beneath it, not silently match nothing.
func TestCodeowners_AnchoredDirectoryPattern(t *testing.T) {
	co := &Codeowners{
		Rules: []CodeownersRule{
			{Pattern: "/internal/cmd/", Owners: []string{"@cli-team"}},
			{Pattern: "/internal/graph", Owners: []string{"@graph-team"}},
		},
	}

	cases := []struct {
		path      string
		wantOwner string
	}{
		{"internal/cmd/analyze.go", "@cli-team"},
		{"internal/cmd/sub/why.go", "@cli-team"},
		{"internal/graph/builder.go", "@graph-team"},
	}
	for _, tc := range cases {
		owners := co.Match(tc.path)
		if len(owners) != 1 || owners[0] != tc.wantOwner {
			t.Errorf("Match(%q) = %v, want [%s]", tc.path, owners, tc.wantOwner)
		}
	}
}
