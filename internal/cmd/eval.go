package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/anurag/tracescope/internal/eval"
	"github.com/spf13/cobra"
)

var (
	evalRepo   string
	evalMax    int
	evalWindow int
	evalDepth  int
	evalSeeds  int
	evalOut    string
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Evaluate the risk ranking against a repo's revert/fix history",
	Long: `Replays TraceScope over a repository's history and measures how well its
risk ranking predicts which changes went wrong.

For each recent integration commit it indexes the tree at that commit, analyzes
the commit's own diff, and records a risk feature. It labels each commit by
whether it was later reverted or hot-fixed, then reports whether the risk ranking
sorts the bad commits to the top — against a random baseline and a changed-lines
(churn) baseline.

Runs against any local git repo; point it at a clone of a larger Go project for a
statistically meaningful sample. Requires git on PATH.

Examples:
  tracescope eval --repo . --max 40
  tracescope eval --repo ../cockroach --max 300 --window 14 --out eval.json`,
	RunE:          runEval,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	evalCmd.Flags().StringVar(&evalRepo, "repo", ".", "path to the git repository to replay")
	evalCmd.Flags().IntVar(&evalMax, "max", 50, "number of most-recent integration commits to evaluate")
	evalCmd.Flags().IntVar(&evalWindow, "window", 14, "days after a commit in which a fix counts as a follow-up")
	evalCmd.Flags().IntVar(&evalDepth, "depth", 5, "blast-radius depth")
	evalCmd.Flags().IntVar(&evalSeeds, "seeds", 20, "random-baseline seeds to average over")
	evalCmd.Flags().StringVar(&evalOut, "out", "", "write the full report as JSON to this path")
	rootCmd.AddCommand(evalCmd)
}

func runEval(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(os.Stderr, "  Replaying up to %d commits from %s ...\n", evalMax, evalRepo)
	report, err := eval.Run(eval.Options{
		Repo:       evalRepo,
		Max:        evalMax,
		WindowDays: evalWindow,
		Depth:      evalDepth,
		Seeds:      evalSeeds,
	})
	if err != nil {
		return err
	}

	printEvalReport(report)

	if evalOut != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling report: %w", err)
		}
		if err := os.WriteFile(evalOut, data, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", evalOut, err)
		}
		fmt.Fprintf(os.Stderr, "\n  Full report written to %s\n", evalOut)
	}
	return nil
}

func printEvalReport(r *eval.Report) {
	out := os.Stderr
	fmt.Fprintf(out, "\n  TraceScope — Risk Ranking Evaluation\n")
	fmt.Fprintf(out, "  Repo: %s\n", r.Repo)
	fmt.Fprintf(out, "  Commits evaluated (n): %d\n", r.N)
	fmt.Fprintf(out, "  Risky (label=1): %d  (%d reverted, %d hot-fixed)  base rate %.1f%%\n",
		r.Positives, r.Reverted, r.HotFixed, pct(r.Positives, r.N))
	fmt.Fprintf(out, "  Fix window: %d days\n", r.WindowDays)

	if r.Positives == 0 {
		fmt.Fprintf(out, "\n  No risky commits found in this window — nothing to rank. Try a larger\n")
		fmt.Fprintf(out, "  --max or a repo with revert/fix history.\n")
		return
	}
	if r.Positives == r.N {
		fmt.Fprintf(out, "\n  Every commit is labeled risky — ranking metrics are degenerate.\n")
	}

	// Header
	fmt.Fprintf(out, "\n  %-10s %8s %8s %8s %8s\n", "ranker", "AUC", "P@5", "P@10", "IFA")
	fmt.Fprintf(out, "  %-10s %8s %8s %8s %8s\n", "------", "---", "---", "----", "---")
	for _, name := range []string{"ladder", "churn", "random"} {
		m := r.Rankers[name]
		fmt.Fprintf(out, "  %-10s %8.3f %8.3f %8.3f %8.1f\n",
			name, m.AUC, m.PrecisionAt[5], m.PrecisionAt[10], m.IFA)
	}
	fmt.Fprintf(out, "\n  AUC 0.5 = no signal; higher is better. P@k = fraction of the top-k\n")
	fmt.Fprintf(out, "  ranked commits that were actually risky. IFA = rank of the first\n")
	fmt.Fprintf(out, "  risky commit (lower is better). n=%d is small — read directionally.\n", r.N)
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}
