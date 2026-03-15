package ownership

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// matchPattern matches a CODEOWNERS glob pattern against a file path.
func matchPattern(pattern, filePath string) bool {
	pattern = filepath.ToSlash(pattern)

	// If pattern starts with /, it's anchored to root — strip the /
	if strings.HasPrefix(pattern, "/") {
		pattern = pattern[1:]
	}

	// Handle ** (matches any number of directories)
	if strings.Contains(pattern, "**") {
		return matchDoublestar(pattern, filePath)
	}

	// If pattern has no /, it matches anywhere in the path
	if !strings.Contains(pattern, "/") {
		// Match against filename or filepath.Match against every segment
		base := filepath.Base(filePath)
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		// Also try full path match
		if matched, _ := filepath.Match(pattern, filePath); matched {
			return true
		}
		return false
	}

	// Pattern has a / — match against full path
	if matched, _ := filepath.Match(pattern, filePath); matched {
		return true
	}

	// Try suffix match (e.g., "docs/*.md" matches "src/docs/readme.md")
	pathParts := strings.Split(filePath, "/")
	patternParts := strings.Split(pattern, "/")
	if len(patternParts) <= len(pathParts) {
		suffix := strings.Join(pathParts[len(pathParts)-len(patternParts):], "/")
		if matched, _ := filepath.Match(pattern, suffix); matched {
			return true
		}
	}

	return false
}

// matchDoublestar handles ** patterns.
func matchDoublestar(pattern, filePath string) bool {
	// Split pattern on **
	parts := strings.SplitN(pattern, "**", 2)
	prefix := strings.TrimRight(parts[0], "/")
	suffix := ""
	if len(parts) > 1 {
		suffix = strings.TrimLeft(parts[1], "/")
	}

	// Check prefix
	if prefix != "" && !strings.HasPrefix(filePath, prefix+"/") && filePath != prefix {
		return false
	}

	// Check suffix
	if suffix == "" {
		return true
	}

	// The suffix must match the end of the path
	remaining := filePath
	if prefix != "" {
		remaining = strings.TrimPrefix(filePath, prefix+"/")
	}

	// Try matching suffix against every possible tail
	segments := strings.Split(remaining, "/")
	for i := 0; i < len(segments); i++ {
		tail := strings.Join(segments[i:], "/")
		if matched, _ := filepath.Match(suffix, tail); matched {
			return true
		}
		// Also try matching just the last part
		if matched, _ := filepath.Match(suffix, segments[len(segments)-1]); matched {
			return true
		}
	}

	return false
}
