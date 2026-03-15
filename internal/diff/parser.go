package diff

import (
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

// ChangedFile represents a file modified in a diff with its changed line ranges.
type ChangedFile struct {
	Path       string      `json:"path"`
	LineRanges []LineRange `json:"line_ranges"`
	IsNew      bool        `json:"is_new,omitempty"`
	IsDeleted  bool        `json:"is_deleted,omitempty"`
}

// LineRange represents a range of changed lines.
type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// ParseUnifiedDiff parses a unified diff and extracts changed files with line ranges.
func ParseUnifiedDiff(diffData []byte) ([]ChangedFile, error) {
	fileDiffs, err := diff.ParseMultiFileDiff(diffData)
	if err != nil {
		return nil, err
	}

	var result []ChangedFile

	for _, fd := range fileDiffs {
		// Skip binary files
		if isBinaryDiff(fd) {
			continue
		}

		cf := ChangedFile{
			Path:      cleanDiffPath(fd.NewName),
			IsNew:     fd.OrigName == "/dev/null",
			IsDeleted: fd.NewName == "/dev/null",
		}

		if cf.IsDeleted {
			cf.Path = cleanDiffPath(fd.OrigName)
		}

		for _, hunk := range fd.Hunks {
			ranges := extractChangedLinesFromBody(hunk)
			cf.LineRanges = append(cf.LineRanges, ranges...)
		}

		result = append(result, cf)
	}

	return result, nil
}

// extractChangedLinesFromBody parses the hunk body line by line.
func extractChangedLinesFromBody(hunk *diff.Hunk) []LineRange {
	var ranges []LineRange

	body := string(hunk.Body)
	line := int(hunk.NewStartLine)
	rangeStart := -1

	lines := splitLines(body)
	// Remove trailing empty string produced by splitLines when body ends with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	for _, bodyLine := range lines {
		if len(bodyLine) == 0 {
			// Empty context line — still advances line counter
			if rangeStart != -1 {
				ranges = append(ranges, LineRange{Start: rangeStart, End: line - 1})
				rangeStart = -1
			}
			line++
			continue
		}

		prefix := bodyLine[0]

		switch prefix {
		case '+':
			if rangeStart == -1 {
				rangeStart = line
			}
			line++
		case '-':
			// Deleted line — doesn't exist in new file
			if rangeStart != -1 {
				ranges = append(ranges, LineRange{Start: rangeStart, End: line - 1})
				rangeStart = -1
			}
		case '\\':
			// No newline marker — skip
		default: // context line (space or anything else)
			if rangeStart != -1 {
				ranges = append(ranges, LineRange{Start: rangeStart, End: line - 1})
				rangeStart = -1
			}
			line++
		}
	}

	// Close any open range
	if rangeStart != -1 {
		ranges = append(ranges, LineRange{Start: rangeStart, End: line - 1})
	}

	return ranges
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// isBinaryDiff checks if a file diff represents a binary file change.
func isBinaryDiff(fd *diff.FileDiff) bool {
	for _, ext := range fd.Extended {
		lower := strings.ToLower(ext)
		if strings.Contains(lower, "binary") || strings.Contains(lower, "git binary patch") {
			return true
		}
	}
	return false
}

func cleanDiffPath(path string) string {
	// Remove a/ or b/ prefix
	if len(path) > 2 && (path[:2] == "a/" || path[:2] == "b/") {
		return path[2:]
	}
	return path
}
