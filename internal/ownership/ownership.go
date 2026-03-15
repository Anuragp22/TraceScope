package ownership

import (
	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/diff"
)

// OwnershipInfo holds combined git blame and CODEOWNERS data.
type OwnershipInfo struct {
	FileAuthors        map[string]*FileAuthorInfo `json:"file_authors"`
	SuggestedReviewers []OwnerSummary             `json:"suggested_reviewers,omitempty"`
	CodeownersFound    bool                       `json:"codeowners_found"`
}

// ResolveOwnership looks up git authors and CODEOWNERS for affected code.
func ResolveOwnership(repoRoot string, affectedFunctions []analyzer.AffectedFunction, changedFiles []diff.ChangedFile) (*OwnershipInfo, error) {
	// Collect all unique file paths
	pathSet := make(map[string]bool)
	for _, af := range affectedFunctions {
		if af.Node != nil && af.Node.FilePath != "" {
			pathSet[af.Node.FilePath] = true
		}
	}
	for _, cf := range changedFiles {
		pathSet[cf.Path] = true
	}

	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}

	// Git log lookup
	fileAuthors := LookupFileAuthors(repoRoot, paths)

	// CODEOWNERS lookup
	var reviewers []OwnerSummary
	codeownersFound := false

	co, err := LoadCodeowners(repoRoot)
	if err == nil && co != nil {
		codeownersFound = true
		// Only suggest reviewers for affected files (not just changed)
		affectedPaths := make([]string, 0, len(affectedFunctions))
		for _, af := range affectedFunctions {
			if af.Node != nil {
				affectedPaths = append(affectedPaths, af.Node.FilePath)
			}
		}
		// Include changed files too
		for _, cf := range changedFiles {
			affectedPaths = append(affectedPaths, cf.Path)
		}
		reviewers = co.SuggestReviewers(affectedPaths)
	}

	return &OwnershipInfo{
		FileAuthors:        fileAuthors,
		SuggestedReviewers: reviewers,
		CodeownersFound:    codeownersFound,
	}, nil
}
