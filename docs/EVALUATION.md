# Evaluating the Risk Ranking

TraceScope ranks the functions a reviewer should look at by a **review score** built
on a categorical **risk ladder** (`internal/analyzer/risk_scorer.go`,
`internal/analyzer/blast_radius.go`). The weights in that score (tier bases 80/50/20,
the caller term, the depth and confidence adjustments) were a hand-written prior. The
obvious question — *"where do those numbers come from, and do they work?"* — deserves a
measured answer, not a shrug. This document is that answer.

The `tracescope eval` subcommand replays the tool over a repository's own history and
measures whether its risk ranking actually sorts the changes that later went wrong to
the top. **The headline: on this corpus the ranking carries only weak signal, and almost
all of that signal is coarse ("does this change touch real functions at all"), not the
fine-grained weighting.** That is an honest, useful result — and it points squarely at
what to build next.

> Reproduce:
> ```
> tracescope eval --repo <path-to-a-go-repo> --max 300 --window 30 --out eval.json
> ```

---

## Methodology

**Replay.** For each of the last `--max` integration commits (first-parent line), the
harness checks out that commit's tree in an isolated `git worktree`, builds a dependency
graph in-process with the **parser backend** (no SCIP — that keeps hundreds of commits
tractable), maps the commit's own diff (`git diff C^1 C`) onto functions, and runs the
normal blast-radius analysis. It records one risk feature per commit (the **max review
score** over affected functions), plus churn, affected count, and max fan-in.

**Labels** (a change "went wrong" if either fires):
- **Reverted** — a later commit's body says `This reverts commit <sha>`.
- **Hot-fixed** — a commit within `--window` days, with a fix-shaped subject
  (`fix|bug|hotfix|patch|revert|regression`), touches files this commit also changed.

This is deliberately simpler than SZZ; it trades some precision for being trivially
mineable and auditable. Its noise is discussed under *Limitations*.

**Metrics**, each reported for three rankers so the number is falsifiable, not asserted:
- **ladder** — rank by TraceScope's review score.
- **churn** — rank by changed-line count. The "surprisingly hard" baseline the
  just-in-time defect-prediction literature repeatedly warns about.
- **random** — averaged over 20 seeds; a sanity floor (should land at AUC ≈ 0.5).

Reported: **AUC** (probability a random risky commit outranks a random clean one;
Mann–Whitney, ties at 0.5), **Precision@k** (fraction of the top-k ranked that were
risky), and **IFA** (rank of the first risky commit; lower is better).

---

## Result (gin-gonic/gin, 300 commits)

n = 300 integration commits · 59 labeled risky (**19.7%** base rate: 2 reverted, 59
hot-fixed) · 30-day window.

| ranker | AUC | P@5 | P@10 | IFA |
|---|---|---|---|---|
| **ladder** | **0.613** | 0.000 | 0.200 | 6.0 |
| churn | 0.595 | 0.000 | 0.000 | 13.0 |
| random | 0.494 | 0.220 | 0.200 | 5.7 |

At face value the ladder wins: AUC 0.613 beats churn (0.595) and clearly beats random
(0.494 — the sanity floor lands where it should). But two things make that headline
misleading, and both are the honest story.

### 1. Two-thirds of commits score zero — and that binary is doing the work

**201 of 300 commits get review score 0**: their diff touched no function the
parser-backend graph resolved (docs, CI, config, test-only changes, or calls the
non-SCIP backend couldn't resolve). Worse for recall, **29 of the 59 risky commits also
score 0** — for half the changes that actually went wrong, the ranking is blind.

Restricting to the 99 commits the tool actually scored (base rate 30.3%), the AUC
**collapses to chance** (derived from the emitted report JSON):

| ranker (scored subset, n=99) | AUC |
|---|---|
| max fan-in | 0.541 |
| review-score sum | 0.528 |
| churn | 0.526 |
| **review score (the ladder)** | **0.516** |

So the full-corpus 0.613 is almost entirely the coarse signal *"this change touches real
functions"* — which is mildly predictive on its own — **not** the carefully-weighted
score. Among commits the ladder rates, its ordering is barely better than a coin flip,
and **raw fan-in (0.541) outranks the composite score (0.516)**. The fine-grained
weighting is not earning its complexity on this corpus.

### 2. Precision@5 = 0 is a real finding, not a bug

The five highest-blast-radius commits were large refactors, features, and chores
(`refactor Keys type to map[...]`, `add OptionFunc/With`, `add ability to override
debugPrintFunc`) — **not** the ones that got hot-fixed. This is exactly what the
change-impact literature predicts: **impact ≠ probability of defect.** Blast radius
measures how *far* a change's effects reach, not how *likely* the change is to be wrong.
Big, wide-reaching changes also get more review attention, which cuts the other way.

---

## Honest reading

- The ranking has **weak-but-real** aggregate signal (AUC ~0.61), and it beats pure
  churn — but only because it encodes a coarse "touches functions" bit. The part I
  hand-tuned — the tier weights, the caller term — adds **no measurable value** here
  over raw fan-in.
- This is the *category error* made concrete. The score models **Impact**
  (`Risk = P(defect) × Impact`), and Impact is a poor standalone defect predictor. The
  weights aren't just unvalidated; the evaluation says they're close to inert.
- **n is small and it's one repo.** Read this directionally: a real conclusion needs
  several hundred labeled commits across multiple repositories.

## What this says to build next (in priority order)

1. **Add the P(defect) axis.** Everything measured here is an Impact metric. The missing
   term — code churn, change entropy, prior fix-commits on the same functions, developer
   recency (the JIT-defect-prediction feature set) — is all derivable from `git log`,
   which the tool already shells out to. That is where the signal is.
2. **Use SCIP in the eval** so far fewer commits score zero; today the parser backend's
   coverage caps recall at ~half the risky commits.
3. **Fit a learned ranker** (a stdlib logistic regression, time-ordered split) over the
   Impact + P(defect) features, once a several-hundred-commit multi-repo corpus makes a
   fitted model meaningful rather than memorized. The per-commit features are already
   emitted by `--out`, so this is a small addition.
4. **Function-level Precision@5** — did the top-5 *affected* functions contain the one a
   targeted follow-up fix actually touched — for corpora with enough targeted fixes.

## Limitations

- **Label noise.** The hot-fix heuristic (fix-shaped subject + file overlap within a
  window) over-labels churny files and misses fixes that don't touch the same files;
  the revert signal is clean but sparse. No SZZ blame-tracing.
- **Parser backend, not SCIP** — chosen for replay speed; it depresses recall (the
  zero-score commits) and would change the scored-subset numbers.
- **Single small corpus** (gin). The harness is repo-agnostic; point `--repo` at a
  clone of any Go project to widen the sample.
- The `random` baseline confirms the pipeline (AUC 0.494 ≈ 0.5); a value far from 0.5
  would indicate a bug in labeling or metrics.
