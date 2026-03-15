package diff

import (
	"github.com/sourcegraph/go-diff/diff"
)

// ChangedFile represents a file modified in a diff with its changed line ranges.
type ChangedFile struct {
	Path       string
	LineRanges []LineRange
	IsNew      bool
	IsDeleted  bool
}

// LineRange represents a range of changed lines.
type LineRange struct {
	Start int
	End   int
}

// ParseUnifiedDiff parses a unified diff and extracts changed files with line ranges.
func ParseUnifiedDiff(diffData []byte) ([]ChangedFile, error) {
	fileDiffs, err := diff.ParseMultiFileDiff(diffData)
	if err != nil {
		return nil, err
	}

	var result []ChangedFile

	for _, fd := range fileDiffs {
		cf := ChangedFile{
			Path:      cleanDiffPath(fd.NewName),
			IsNew:     fd.OrigName == "/dev/null",
			IsDeleted: fd.NewName == "/dev/null",
		}

		if cf.IsDeleted {
			cf.Path = cleanDiffPath(fd.OrigName)
		}

		for _, hunk := range fd.Hunks {
			ranges := extractChangedLines(hunk)
			cf.LineRanges = append(cf.LineRanges, ranges...)
		}

		result = append(result, cf)
	}

	return result, nil
}

// extractChangedLines extracts the new-file line ranges that were changed in a hunk.
func extractChangedLines(hunk *diff.Hunk) []LineRange {
	var ranges []LineRange

	line := int(hunk.NewStartLine)
	rangeStart := -1

	for _, b := range hunk.Body {
		switch b {
		case '+':
			if rangeStart == -1 {
				rangeStart = line
			}
		case '-':
			// Deleted lines don't advance the new-file line counter
			continue
		case '\\':
			// "\ No newline at end of file" — skip
			continue
		default: // context line (space)
			if rangeStart != -1 {
				ranges = append(ranges, LineRange{Start: rangeStart, End: line - 1})
				rangeStart = -1
			}
		}

		// Advance line for non-delete lines
		if b != '-' {
			// Find next newline to advance
		}
	}

	// We need a more robust line-by-line approach
	ranges = ranges[:0]
	return extractChangedLinesFromBody(hunk)
}

// extractChangedLinesFromBody parses the hunk body line by line.
func extractChangedLinesFromBody(hunk *diff.Hunk) []LineRange {
	var ranges []LineRange

	body := string(hunk.Body)
	line := int(hunk.NewStartLine)
	rangeStart := -1

	for _, bodyLine := range splitLines(body) {
		if len(bodyLine) == 0 {
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

func cleanDiffPath(path string) string {
	// Remove a/ or b/ prefix
	if len(path) > 2 && (path[:2] == "a/" || path[:2] == "b/") {
		return path[2:]
	}
	return path
}
