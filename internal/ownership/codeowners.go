package ownership

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// CodeownersRule is a single pattern → owners mapping.
type CodeownersRule struct {
	Pattern string
	Owners  []string
}

// Codeowners holds parsed CODEOWNERS rules.
type Codeowners struct {
	Rules []CodeownersRule
}

// OwnerSummary aggregates an owner's affected files.
type OwnerSummary struct {
	Owner     string   `json:"owner"`
	FilePaths []string `json:"file_paths"`
	FileCount int      `json:"file_count"`
}

// LoadCodeowners searches for a CODEOWNERS file in standard locations.
func LoadCodeowners(repoRoot string) (*Codeowners, error) {
	locations := []string{
		filepath.Join(repoRoot, "CODEOWNERS"),
		filepath.Join(repoRoot, ".github", "CODEOWNERS"),
		filepath.Join(repoRoot, "docs", "CODEOWNERS"),
	}

	for _, loc := range locations {
		f, err := os.Open(loc)
		if err != nil {
			continue
		}
		defer f.Close()
		return parseCodeowners(f)
	}

	return nil, nil // no CODEOWNERS file found — not an error
}

func parseCodeowners(f *os.File) (*Codeowners, error) {
	var rules []CodeownersRule
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		pattern := fields[0]
		owners := fields[1:]

		rules = append(rules, CodeownersRule{
			Pattern: pattern,
			Owners:  owners,
		})
	}

	return &Codeowners{Rules: rules}, scanner.Err()
}

// Match returns the owners for a file path. Last matching rule wins (CODEOWNERS semantics).
func (c *Codeowners) Match(filePath string) []string {
	if c == nil {
		return nil
	}
	normalized := filepath.ToSlash(filePath)

	var matched []string
	for _, rule := range c.Rules {
		if matchPattern(rule.Pattern, normalized) {
			matched = rule.Owners
		}
	}
	return matched
}

// SuggestReviewers aggregates owners across affected files, sorted by file count.
func (c *Codeowners) SuggestReviewers(filePaths []string) []OwnerSummary {
	if c == nil {
		return nil
	}

	ownerFiles := make(map[string][]string)
	for _, fp := range filePaths {
		owners := c.Match(fp)
		for _, owner := range owners {
			ownerFiles[owner] = append(ownerFiles[owner], fp)
		}
	}

	var result []OwnerSummary
	for owner, files := range ownerFiles {
		result = append(result, OwnerSummary{
			Owner:     owner,
			FilePaths: files,
			FileCount: len(files),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].FileCount != result[j].FileCount {
			return result[i].FileCount > result[j].FileCount
		}
		return result[i].Owner < result[j].Owner
	})

	return result
}

// matchPattern matches a CODEOWNERS glob pattern against a repo-relative file path.
func matchPattern(pattern, filePath string) bool {
	pattern = filepath.ToSlash(pattern)
	filePath = filepath.ToSlash(filePath)

	// A leading slash anchors the pattern to the repo root; our file paths are
	// already repo-relative, so the slash is only a marker — strip it.
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	if pattern == "" {
		return false
	}

	// Build the candidate globs. A trailing slash means "this directory and
	// everything under it"; a bare path may name either a file or a directory.
	var globs []string
	if strings.HasSuffix(pattern, "/") {
		globs = []string{pattern + "**"}
	} else {
		globs = []string{pattern, pattern + "/**"}
	}
	for _, g := range globs {
		if matched, _ := doublestar.Match(g, filePath); matched {
			return true
		}
	}

	// A pattern with no slash matches that name anywhere in the path
	// (CODEOWNERS semantics for un-anchored patterns).
	if !anchored && !strings.Contains(pattern, "/") && !strings.Contains(pattern, "**") {
		if matched, _ := doublestar.Match(pattern, filepath.Base(filePath)); matched {
			return true
		}
	}

	return false
}
