package cmd

import (
	"os/exec"
	"path/filepath"
	"strings"

	diffpkg "github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
	"github.com/bmatcuk/doublestar/v4"
)

// findRepoRoot uses git to find the repository root.
func findRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// gitHead returns the HEAD commit SHA for the repo at dir, or "" if unavailable.
func gitHead(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitOriginRemote returns the normalized origin remote for the repo at dir,
// or "" if there is no origin (e.g. a local-only repo).
func gitOriginRemote(dir string) string {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return normalizeRemote(strings.TrimSpace(string(out)))
}

// shortSHA truncates a commit hash to 7 characters for display.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// normalizeRemote canonicalizes a git remote URL to "host/owner/repo" so that
// https, ssh, and .git-suffixed forms of the same repo compare equal.
func normalizeRemote(url string) string {
	u := strings.TrimSpace(url)
	if u == "" {
		return ""
	}
	u = strings.TrimSuffix(u, ".git")
	// scp-like: git@github.com:owner/repo
	if strings.HasPrefix(u, "git@") {
		u = strings.TrimPrefix(u, "git@")
		u = strings.Replace(u, ":", "/", 1)
		return u
	}
	// scheme://[user@]host/owner/repo
	if i := strings.Index(u, "://"); i != -1 {
		u = u[i+3:]
		if at := strings.LastIndex(u, "@"); at != -1 {
			u = u[at+1:]
		}
	}
	return u
}

// loadGraph loads the dependency graph from the standard or config-specified path.
func loadGraph() (*graph.GraphData, error) {
	cwd, _ := filepath.Abs(".")
	graphFile := filepath.Join(cwd, ".tracescope", "graph.json")
	if cfg.GraphPath != "" {
		graphFile = cfg.GraphPath
		if !filepath.IsAbs(graphFile) {
			graphFile = filepath.Join(cwd, graphFile)
		}
	}
	store := graph.NewStore()
	return store.Load(graphFile)
}

// filterIgnoredFiles removes files matching any of the comma-separated glob patterns.
func filterIgnoredFiles(files []diffpkg.ChangedFile, patterns string) []diffpkg.ChangedFile {
	globs := splitGlobs(patterns)

	var result []diffpkg.ChangedFile
	for _, f := range files {
		if matchesAnyGlob(f.Path, globs) {
			continue
		}
		result = append(result, f)
	}
	return result
}

// splitGlobs splits a comma-separated pattern string into trimmed globs.
func splitGlobs(patterns string) []string {
	parts := strings.Split(patterns, ",")
	globs := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			globs = append(globs, p)
		}
	}
	return globs
}

// matchesAnyGlob checks if a path matches any of the glob patterns using doublestar.
func matchesAnyGlob(path string, globs []string) bool {
	normalized := filepath.ToSlash(path)
	for _, g := range globs {
		if g == "" {
			continue
		}
		if matched, _ := doublestar.Match(g, normalized); matched {
			return true
		}
		// Also try matching against just the filename for simple patterns
		if !strings.Contains(g, "/") {
			if matched, _ := doublestar.Match(g, filepath.Base(normalized)); matched {
				return true
			}
		}
	}
	return false
}

// normalizeLang converts short language names to the canonical forms used in the graph.
func normalizeLang(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "js", "javascript":
		return "javascript"
	case "ts", "typescript":
		return "typescript"
	case "py", "python":
		return "python"
	case "go", "golang":
		return "go"
	default:
		return strings.ToLower(strings.TrimSpace(lang))
	}
}

// parseLangFilter parses a comma-separated language filter into a set.
// Returns nil if no filter (meaning "all languages").
func parseLangFilter(langFlag string) map[string]bool {
	if langFlag == "" {
		return nil
	}
	parts := strings.Split(langFlag, ",")
	filter := make(map[string]bool, len(parts))
	for _, p := range parts {
		normalized := normalizeLang(p)
		if normalized != "" {
			filter[normalized] = true
		}
	}
	return filter
}
