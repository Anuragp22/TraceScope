package cmd

import "testing"

func TestNormalizeRemote(t *testing.T) {
	cases := map[string]string{
		"https://github.com/anurag/tracescope.git":   "github.com/anurag/tracescope",
		"https://github.com/anurag/tracescope":       "github.com/anurag/tracescope",
		"git@github.com:anurag/tracescope.git":       "github.com/anurag/tracescope",
		"git@github.com:anurag/tracescope":           "github.com/anurag/tracescope",
		"ssh://git@github.com/anurag/tracescope.git": "github.com/anurag/tracescope",
		"https://user@gitlab.com/group/sub/repo.git": "gitlab.com/group/sub/repo",
		"": "",
	}
	for in, want := range cases {
		if got := normalizeRemote(in); got != want {
			t.Errorf("normalizeRemote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShortSHA(t *testing.T) {
	if got := shortSHA("2c2d5e0abc123"); got != "2c2d5e0" {
		t.Errorf("shortSHA long = %q, want 2c2d5e0", got)
	}
	if got := shortSHA("abc"); got != "abc" {
		t.Errorf("shortSHA short = %q, want abc", got)
	}
}
