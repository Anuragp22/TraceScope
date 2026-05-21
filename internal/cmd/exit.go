package cmd

import (
	"errors"

	"github.com/anurag/tracescope/internal/analyzer"
)

// ResolveExit maps an error returned by Execute to a process exit code and the
// message to print on stderr (empty when nothing should be printed).
//
// Exit codes:
//
//	0 — success / no significant risk
//	1 — high-risk impact   (analyzer.RiskExitError)
//	2 — medium-risk impact (analyzer.RiskExitError)
//	3 — a TraceScope error (bad input, missing graph, etc.)
//
// Code 3 keeps genuine failures distinct from the risk gate: a broken
// invocation must not be mistaken for a high-risk PR in CI. Errors are never
// silent — every non-risk failure returns a user-visible message.
func ResolveExit(err error) (code int, message string) {
	if err == nil {
		return 0, ""
	}
	var riskErr *analyzer.RiskExitError
	if errors.As(err, &riskErr) {
		return riskErr.Code, ""
	}
	return 3, "Error: " + err.Error()
}
