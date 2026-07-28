package ownership

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// FileAuthorInfo holds the last author info for a file.
type FileAuthorInfo struct {
	FilePath     string    `json:"file_path"`
	LastAuthor   string    `json:"last_author"`
	LastEmail    string    `json:"last_email"`
	LastModified time.Time `json:"last_modified"`
}

// LookupFileAuthors runs git log to find the last author for each file.
// Runs concurrently with a bounded worker pool. Skips files that fail.
func LookupFileAuthors(repoRoot string, filePaths []string) map[string]*FileAuthorInfo {
	// Deduplicate
	unique := make(map[string]bool)
	var deduped []string
	for _, fp := range filePaths {
		if !unique[fp] {
			unique[fp] = true
			deduped = append(deduped, fp)
		}
	}

	result := make(map[string]*FileAuthorInfo)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Bounded worker pool
	workers := 8
	if len(deduped) < workers {
		workers = len(deduped)
	}
	if workers == 0 {
		return result
	}

	jobCh := make(chan string, len(deduped))
	for _, fp := range deduped {
		jobCh <- fp
	}
	close(jobCh)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for fp := range jobCh {
				info := gitLogLastAuthor(repoRoot, fp)
				if info != nil {
					mu.Lock()
					result[fp] = info
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	return result
}

func gitLogLastAuthor(repoRoot, filePath string) *FileAuthorInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Separate fields with 0x1f rather than "|": an author name may legitimately
	// contain a pipe, which shifted every later field and left the date unparseable.
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--format=%an\x1f%ae\x1f%aI", "--", filePath)
	cmd.Dir = repoRoot

	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return nil
	}

	parts := strings.SplitN(line, "\x1f", 3)
	if len(parts) != 3 {
		return nil
	}

	// %aI is strict ISO-8601, which is RFC3339. A parse failure means the field
	// was not a date at all, so leave the zero time — RelativeTime renders it
	// as "unknown" rather than inventing a timestamp.
	modified, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[2]))
	if err != nil {
		modified = time.Time{}
	}

	return &FileAuthorInfo{
		FilePath:     filePath,
		LastAuthor:   strings.TrimSpace(parts[0]),
		LastEmail:    strings.TrimSpace(parts[1]),
		LastModified: modified,
	}
}

// RelativeTime formats a time as a human-readable relative string.
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%d minute%s ago", m, plural(m))
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%d hour%s ago", h, plural(h))
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d day%s ago", days, plural(days))
	case d < 365*24*time.Hour:
		months := int(d.Hours() / 24 / 30)
		return fmt.Sprintf("%d month%s ago", months, plural(months))
	default:
		years := int(d.Hours() / 24 / 365)
		return fmt.Sprintf("%d year%s ago", years, plural(years))
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
