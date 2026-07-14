// Package eval replays TraceScope over a repository's history to measure how well
// its risk ranking predicts which changes actually went wrong.
//
// For each integration commit C (a merge or a direct commit on the main line) it
// indexes the tree at C in an isolated git worktree, maps C's own diff onto
// functions, runs the blast-radius analysis, and records a per-commit risk
// feature. It then labels each commit by whether it was later reverted or
// hot-fixed, and reports whether the risk ranking sorts the bad commits to the
// top — compared against a random baseline and a changed-lines (churn) baseline.
//
// The whole point is to make the hand-tuned risk score falsifiable: the metrics
// come with a random and a churn baseline so "the ladder beats them" (or doesn't)
// is visible, not asserted. Labels use revert / follow-up-fix mining rather than
// SZZ; see docs/EVALUATION.md for the methodology and its limits.
package eval

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/parser"
)

// Options configures an evaluation run.
type Options struct {
	Repo       string // path to the git repository to replay
	Max        int    // number of most-recent integration commits to consider
	WindowDays int    // days after a commit in which a fix counts as a follow-up
	Depth      int    // blast-radius depth (default 5)
	Seeds      int    // random-baseline seeds to average over
}

// Commit is one replayed integration commit with its risk features and label.
type Commit struct {
	SHA          string   `json:"sha"`
	Subject      string   `json:"subject"`
	UnixTime     int64    `json:"unix_time"`
	ChangedFiles []string `json:"-"`
	// Features
	Churn            int `json:"churn"`              // added + deleted lines
	LadderScore      int `json:"ladder_score"`       // max review score over affected fns
	LadderScoreSum   int `json:"ladder_score_sum"`   // sum of review scores
	AffectedCount    int `json:"affected_count"`     //
	ChangedFuncCount int `json:"changed_func_count"` //
	MaxFanIn         int `json:"max_fan_in"`         // max caller_count among affected
	// Label
	Reverted bool `json:"reverted"`
	HotFixed bool `json:"hot_fixed"`
	Label    int  `json:"label"` // 1 if reverted or hot-fixed, else 0
}

// RankerMetrics holds ranking-quality metrics for one ranker.
type RankerMetrics struct {
	AUC         float64         `json:"auc"`
	PrecisionAt map[int]float64 `json:"precision_at"`
	IFA         float64         `json:"ifa"` // rank of first true positive (1-based); N+1 if none
}

// Report is the full evaluation result.
type Report struct {
	Repo       string                   `json:"repo"`
	N          int                      `json:"n"`
	Positives  int                      `json:"positives"`
	Reverted   int                      `json:"reverted"`
	HotFixed   int                      `json:"hot_fixed"`
	WindowDays int                      `json:"window_days"`
	Ks         []int                    `json:"ks"`
	Rankers    map[string]RankerMetrics `json:"rankers"`
	Commits    []Commit                 `json:"commits"`
}

var (
	revertMsgRe = regexp.MustCompile(`(?m)This reverts commit ([0-9a-f]{7,40})`)
	fixSubjRe   = regexp.MustCompile(`(?i)\b(fix|fixes|fixed|bug|hotfix|patch|revert|regression)\b`)
)

// Run executes the evaluation and returns the report.
func Run(opts Options) (*Report, error) {
	if opts.Max <= 0 {
		opts.Max = 50
	}
	if opts.WindowDays <= 0 {
		opts.WindowDays = 14
	}
	if opts.Depth <= 0 {
		opts.Depth = 5
	}
	if opts.Seeds <= 0 {
		opts.Seeds = 20
	}

	commits, err := listCommits(opts.Repo, opts.Max)
	if err != nil {
		return nil, fmt.Errorf("listing commits: %w", err)
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("no commits with a parent found in %s", opts.Repo)
	}

	// Label first (cheap, pure git) so we can skip replay for nothing.
	labelCommits(opts, commits)

	// Replay each commit: index at its tree, analyze its own diff.
	scorer := &analyzer.RiskScorer{}
	for i := range commits {
		c := &commits[i]
		if err := replayCommit(opts, c, scorer); err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", short(c.SHA), err)
		}
	}

	report := &Report{
		Repo:       opts.Repo,
		N:          len(commits),
		WindowDays: opts.WindowDays,
		Ks:         []int{5, 10},
		Rankers:    map[string]RankerMetrics{},
		Commits:    commits,
	}
	for _, c := range commits {
		if c.Label == 1 {
			report.Positives++
		}
		if c.Reverted {
			report.Reverted++
		}
		if c.HotFixed {
			report.HotFixed++
		}
	}

	report.Rankers["ladder"] = scoreRanker(commits, report.Ks, func(c Commit) float64 { return float64(c.LadderScore) })
	report.Rankers["churn"] = scoreRanker(commits, report.Ks, func(c Commit) float64 { return float64(c.Churn) })
	report.Rankers["random"] = randomRanker(commits, report.Ks, opts.Seeds)

	return report, nil
}

// listCommits returns the most-recent Max integration commits (first-parent line)
// that have at least one parent, newest first.
func listCommits(repo string, max int) ([]Commit, error) {
	// Records separated by 0x1e, fields by 0x1f: sha, unixtime, parents, subject.
	out, err := gitOutput(repo, "log", "--first-parent", "-n", strconv.Itoa(max),
		"--format=%H\x1f%ct\x1f%P\x1f%s\x1e")
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.Split(rec, "\x1f")
		if len(f) < 4 {
			continue
		}
		parents := strings.Fields(f[2])
		if len(parents) == 0 {
			continue // root commit, no diff
		}
		ts, _ := strconv.ParseInt(f[1], 10, 64)
		commits = append(commits, Commit{SHA: f[0], UnixTime: ts, Subject: f[3]})
	}
	return commits, nil
}

// labelCommits marks each commit reverted (a later commit's body reverts it) or
// hot-fixed (a fix-shaped commit within the window touches overlapping files).
func labelCommits(opts Options, commits []Commit) {
	// Reverted set: mine bodies of a generous window of recent commits.
	reverted := revertedSHAs(opts.Repo, opts.Max*3)
	byPrefix := func(sha string) bool {
		for r := range reverted {
			if strings.HasPrefix(sha, r) || strings.HasPrefix(r, sha) {
				return true
			}
		}
		return false
	}

	// Changed files per commit (needed for both hot-fix overlap and revert n/a).
	for i := range commits {
		commits[i].ChangedFiles = changedFiles(opts.Repo, commits[i].SHA)
	}

	windowSec := int64(opts.WindowDays) * 86400
	for i := range commits {
		c := &commits[i]
		if byPrefix(c.SHA) {
			c.Reverted = true
		}
		// Hot-fix: any *other* commit landing within the window after c, with a
		// fix-shaped subject and overlapping changed files.
		for j := range commits {
			if i == j {
				continue
			}
			f := &commits[j]
			if f.UnixTime <= c.UnixTime || f.UnixTime > c.UnixTime+windowSec {
				continue
			}
			if !fixSubjRe.MatchString(f.Subject) {
				continue
			}
			if filesOverlap(c.ChangedFiles, f.ChangedFiles) {
				c.HotFixed = true
				break
			}
		}
		if c.Reverted || c.HotFixed {
			c.Label = 1
		}
	}
}

// replayCommit indexes the tree at c in an isolated worktree and analyzes c's diff.
func replayCommit(opts Options, c *Commit, scorer *analyzer.RiskScorer) error {
	// c's own diff (first parent -> c), in c's new-file coordinates.
	diffData, err := gitBytes(opts.Repo, "diff", c.SHA+"^1", c.SHA)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	c.Churn = numstatChurn(opts.Repo, c.SHA)
	changed, err := diff.ParseUnifiedDiff(diffData)
	if err != nil {
		return fmt.Errorf("parse diff: %w", err)
	}
	if len(changed) == 0 {
		return nil // nothing analyzable; leave features at zero
	}

	gd, cleanup, err := buildGraphAt(opts.Repo, c.SHA)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}

	result := analyzer.NewBlastRadiusAnalyzer(gd, opts.Depth, scorer).Analyze(changed)
	c.ChangedFuncCount = len(result.ChangedFunctions)
	c.AffectedCount = len(result.AffectedFunctions)
	for _, af := range result.AffectedFunctions {
		if af.ReviewScore > c.LadderScore {
			c.LadderScore = af.ReviewScore
		}
		c.LadderScoreSum += af.ReviewScore
		if af.CallerCount > c.MaxFanIn {
			c.MaxFanIn = af.CallerCount
		}
	}
	return nil
}

// buildGraphAt checks out the tree at sha in a throwaway worktree and builds a
// parser-backend graph from it (no SCIP — fast enough to replay many commits).
func buildGraphAt(repo, sha string) (*graph.GraphData, func(), error) {
	wt, err := os.MkdirTemp("", "tracescope-eval-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_, _ = gitOutput(repo, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(wt)
	}
	if _, err := gitOutput(repo, "worktree", "add", "--detach", "--force", wt, sha); err != nil {
		return nil, cleanup, fmt.Errorf("worktree add: %w", err)
	}
	filesByLang, err := parser.WalkDirectory(wt)
	if err != nil {
		return nil, cleanup, fmt.Errorf("walk: %w", err)
	}
	results, _ := parser.NewRegistry().ParseFiles(filesByLang)
	gd := graph.NewBuilder().Build(results)
	return gd, cleanup, nil
}

// --- ranking + metrics -------------------------------------------------------

func scoreRanker(commits []Commit, ks []int, score func(Commit) float64) RankerMetrics {
	scores := make([]float64, len(commits))
	labels := make([]int, len(commits))
	for i, c := range commits {
		scores[i] = score(c)
		labels[i] = c.Label
	}
	order := rankOrder(scores)
	m := RankerMetrics{
		AUC:         auc(scores, labels),
		PrecisionAt: map[int]float64{},
		IFA:         ifa(order, labels),
	}
	for _, k := range ks {
		m.PrecisionAt[k] = precisionAtK(order, labels, k)
	}
	return m
}

// randomRanker averages metrics over Seeds deterministic random orderings.
func randomRanker(commits []Commit, ks []int, seeds int) RankerMetrics {
	labels := make([]int, len(commits))
	for i, c := range commits {
		labels[i] = c.Label
	}
	agg := RankerMetrics{PrecisionAt: map[int]float64{}}
	for s := 0; s < seeds; s++ {
		scores := make([]float64, len(commits))
		st := uint64(s)*2862933555777941757 + 3037000493
		for i := range scores {
			st = st*6364136223846793005 + 1442695040888963407
			scores[i] = float64(st >> 11) // deterministic pseudo-random
		}
		order := rankOrder(scores)
		agg.AUC += auc(scores, labels)
		agg.IFA += ifa(order, labels)
		for _, k := range ks {
			agg.PrecisionAt[k] += precisionAtK(order, labels, k)
		}
	}
	f := float64(seeds)
	agg.AUC /= f
	agg.IFA /= f
	for _, k := range ks {
		agg.PrecisionAt[k] /= f
	}
	return agg
}

// rankOrder returns commit indices sorted by score descending (stable on index).
func rankOrder(scores []float64) []int {
	idx := make([]int, len(scores))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return scores[idx[a]] > scores[idx[b]] })
	return idx
}

func precisionAtK(order []int, labels []int, k int) float64 {
	if k > len(order) {
		k = len(order)
	}
	if k == 0 {
		return 0
	}
	hits := 0
	for i := 0; i < k; i++ {
		if labels[order[i]] == 1 {
			hits++
		}
	}
	return float64(hits) / float64(k)
}

// ifa is the 1-based rank of the first true positive, or N+1 if there is none.
func ifa(order []int, labels []int) float64 {
	for i, idx := range order {
		if labels[idx] == 1 {
			return float64(i + 1)
		}
	}
	return float64(len(order) + 1)
}

// auc is the probability a random positive outranks a random negative, via the
// Mann-Whitney identity with 0.5 credit for ties. Returns 0.5 when undefined.
func auc(scores []float64, labels []int) float64 {
	var pos, neg []float64
	for i, l := range labels {
		if l == 1 {
			pos = append(pos, scores[i])
		} else {
			neg = append(neg, scores[i])
		}
	}
	if len(pos) == 0 || len(neg) == 0 {
		return 0.5
	}
	var concordant float64
	for _, p := range pos {
		for _, n := range neg {
			switch {
			case p > n:
				concordant++
			case p == n:
				concordant += 0.5
			}
		}
	}
	return concordant / float64(len(pos)*len(neg))
}

// --- git plumbing ------------------------------------------------------------

func revertedSHAs(repo string, n int) map[string]struct{} {
	out, err := gitOutput(repo, "log", "-n", strconv.Itoa(n), "--format=%B\x1e")
	set := map[string]struct{}{}
	if err != nil {
		return set
	}
	for _, m := range revertMsgRe.FindAllStringSubmatch(out, -1) {
		set[m[1]] = struct{}{}
	}
	return set
}

func changedFiles(repo, sha string) []string {
	out, err := gitOutput(repo, "diff", "--name-only", sha+"^1", sha)
	if err != nil {
		return nil
	}
	var files []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files
}

func numstatChurn(repo, sha string) int {
	out, err := gitOutput(repo, "diff", "--numstat", sha+"^1", sha)
	if err != nil {
		return 0
	}
	total := 0
	for _, l := range strings.Split(out, "\n") {
		fields := strings.Fields(l)
		if len(fields) < 2 {
			continue
		}
		add, aerr := strconv.Atoi(fields[0]) // "-" for binary → error → skip
		del, derr := strconv.Atoi(fields[1])
		if aerr == nil {
			total += add
		}
		if derr == nil {
			total += del
		}
	}
	return total
}

func filesOverlap(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, x := range a {
		set[x] = struct{}{}
	}
	for _, y := range b {
		if _, ok := set[y]; ok {
			return true
		}
	}
	return false
}

func gitOutput(repo string, args ...string) (string, error) {
	b, err := gitBytes(repo, args...)
	return string(b), err
}

func gitBytes(repo string, args ...string) ([]byte, error) {
	full := append([]string{"-C", repo}, args...)
	out, err := exec.Command("git", full...).Output()
	// git diff exits 1 when there are differences; tolerate as long as we got output.
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return out, nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
