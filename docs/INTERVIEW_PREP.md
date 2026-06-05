# TraceScope — Interview Prep

> Everything here is grounded in the code. Open the cited files to verify before you say it out loud.
> Inferred (not code-stated) rationale is marked **[ASSUMED]**.

---

## The pitches

### 30-second pitch
> TraceScope is a static-analysis CLI that does **PR blast-radius analysis for Go**. Code review normally shows you *what* changed; TraceScope shows you what might *break downstream*. It builds a dependency graph of the whole repo — files, functions, classes, and the calls between them — using a compiler-grade SCIP index with a tree-sitter parser as fallback. Then it maps a diff onto the changed functions, walks the graph backwards to find every caller that could be affected, scores each one for risk, and posts a ranked, reviewer-focused comment on the PR. It's wired into CI and exits with a risk-based status code so a high-risk change can gate the merge.

### 2-minute pitch
> The problem I started from: a diff tells you the lines that changed, but not the impact. A one-line change to a widely-called helper can be far riskier than a 200-line change to a leaf function, and reviewers can't see that from the diff alone.
>
> So TraceScope models the repo as a **dependency graph**. The interesting part is that it builds that graph two different ways into one shared model. The precise path ingests a **SCIP index** — that's Sourcegraph's compiler-grade code-intelligence format, produced by `scip-go` — so the call edges are accurate across files and packages. The portable path is a **tree-sitter / `go/ast` parser fallback** that resolves calls with name-and-path heuristics when no index is available. Every edge is tagged with a **confidence** — exact or heuristic — and the graph even records what it *failed* to resolve, so it's honest about its own precision.
>
> Given a diff from stdin, it parses the unified diff, maps the changed line ranges onto the function nodes whose source spans overlap, then runs a **depth-limited reverse BFS** — starting from the changed functions and walking to everyone who calls them — to compute the blast radius. Each affected function gets a **risk label** (HIGH/MEDIUM/LOW) based on how many production callers it has, whether it's exported, and how close it is to the change, plus a separate numeric **review score** that ranks the report. Confidence propagates as the weakest link along each path, so a result that depended on a guessed edge is marked less trustworthy.
>
> The output is a GitHub PR comment with reviewer focus and suggested reviewers from CODEOWNERS and git history, an idempotent update so re-runs don't spam, and an **exit code** that lets CI block on high risk — while keeping tool failures on a distinct code so a crash never looks like a dangerous PR. It dogfoods itself: every PR on the repo gets analyzed by the tool itself.

---

## What's distinctive (vs a typical CRUD app)

A typical CRUD app reads and writes rows. None of the following would appear in one — these are the things to lead with:

1. **Two graph backends measured against each other.** The same `GraphData` model is built from a SCIP index ([scip.go](../internal/graph/scip.go), all `EXACT` edges) *and* from a heuristic parser ([builder.go](../internal/graph/builder.go)), and there's a dedicated `validate-scip` command + [compare.go](../internal/graph/compare.go) that diffs the two graphs by content signature and reports shared/missing/extra edges. The project ships its own **differential quality harness**. Benchmark on `gin-gonic/gin`: parser=3996 edges, SCIP=5667, 3460 shared ([docs/benchmark-real-repo.md](benchmark-real-repo.md)).

2. **Confidence-aware, self-auditing graph.** Every edge is `EXACT` or `HEURISTIC` ([types.go:47](../internal/graph/types.go:47)); the graph records `ResolutionStats` and per-reference `ResolutionIssues` ([types.go:54-77](../internal/graph/types.go:54)) — counts of ambiguous/unresolved calls. The tool tells you where its own precision dropped instead of pretending the call graph is perfect.

3. **Confidence propagates as the weakest link.** An affected function's confidence is the *least* confident hop on its path from the change ([mergeConfidence, query.go:125](../internal/graph/query.go:125)), and heuristic results are penalized `-8` in the ranking score ([blast_radius.go:272](../internal/analyzer/blast_radius.go:272)). The result quantifies *how much to trust each impact claim*.

4. **A hybrid SCIP + native-parser trick** to get function body bounds right (see "Hardest problem" below) — trust the indexer for symbol *identity*, but re-run `go/ast` for accurate *body extents* ([scip.go:257-287](../internal/graph/scip.go:257)).

5. **Determinism treated as a correctness property.** Go map iteration is randomized, which would make CI output flap. So there are sorted symbol iterations with explicit comments ([scip.go:221-227](../internal/graph/scip.go:221)), and every ranked list has a unique final tiebreaker — affected functions break ties on node ID ([blast_radius.go:199](../internal/analyzer/blast_radius.go:199)). Stable output is the whole point of a CI gate.

6. **A real CI contract via exit codes.** `0/1/2/3` where `3` (tool failure) is deliberately *distinct* from `1` (HIGH risk) so "the tool broke" can never be mistaken for "this PR is dangerous" ([exit.go:19-21](../internal/cmd/exit.go:19)). Risk flows up as a typed error mapped in exactly one place.

7. **Idempotent PR commenting.** A hidden HTML marker ([github.go:15](../internal/output/github.go:15)) lets a re-run *update* the existing comment instead of posting a new one each push — driven through the `gh` CLI so the tool never parses the repo slug or handles tokens itself.

8. **Graceful degradation everywhere.** Partial Go ASTs are kept on syntax errors and surfaced ([golang.go:166](../internal/parser/golang.go:166)); a corrupt parse cache self-heals to empty; missing `gh`/`node`/CODEOWNERS are non-fatal; `git log` timeouts skip one file rather than failing the run.

---

## Hardest technical problem

**The problem:** A blast radius is only as trustworthy as the call graph under it, and getting an accurate call graph is genuinely hard. The single thorniest instance: **`scip-go` reports function definitions as identifier-only ranges** — the definition's `EndLine` equals its `StartLine` (just the name token), not the function body. That breaks the core operation in two ways:

- **Diff mapping misses body changes.** TraceScope maps a diff by testing whether changed line ranges *overlap* a function's `[StartLine, EndLine]` span ([diff_mapper.go:66-76](../internal/analyzer/diff_mapper.go:66)). If a function's span is a single line (its signature), a change to *line 40 of its body* overlaps nothing — the changed function is never detected, so its blast radius is empty. Silent false-negative.
- **Reference attribution misfires.** Synthesizing a `CALLS` edge requires knowing which function *contains* a reference line. With identifier-only ranges you can't tell, so a call gets attributed to the wrong caller.

**Why it's hard:** You can't just throw away SCIP — its cross-file/cross-package *symbol resolution* is exactly the thing the parser can't do well, and it's why Go is the reliable language. But its *ranges* are wrong for this use case. And re-deriving symbol identity yourself defeats the purpose of using SCIP at all.

**The solution — hybrid, layered:**

1. **Prefer the enclosing range when SCIP provides one** ([scip.go:176-180](../internal/graph/scip.go:176)): newer `scip-go`/`scip-typescript` emit a fuller body span as the *enclosing range*; adopt its end line when it's larger.
2. **Re-run the native Go parser to fix the rest** ([refineGoFunctionBounds, scip.go:257-287](../internal/graph/scip.go:257)): group SCIP function nodes by file, re-parse each Go source with `go/ast` (which has exact body bounds), build a `startLine → endLine` map, and widen each SCIP node's `EndLine` to the real body end. **Trust SCIP for symbol identity; trust `go/ast` for body extents.**
3. **Attribute references by scope** ([scip.go:418-451, 663-680](../internal/graph/scip.go:418)): precompute function scopes sorted by descending start line / ascending end line, and for each reference find the innermost containing function — falling back to nearest-preceding-definition only when a body range isn't known.
4. **Guard the traversal against the *other* SCIP quirk** ([query.go:48-57](../internal/graph/query.go:48)): SCIP also emits `CALLS` edges from a function to the *types* it references (e.g. a `*Context` param). In reverse BFS, a popular type would become a hub flooding the radius with every type user — so a `CALLS` edge is only followed in reverse when its target is a `function`. (Real bug, fixed in commit `f84e4cd`.)

**The lesson to tell:** the elegant move is recognizing that "use SCIP" isn't all-or-nothing — you can take the part it's good at (identity, cross-file resolution) and patch the part it's weak at (body ranges) with the cheaper, already-present native parser, then verify the whole thing with the `validate-scip` differential harness. The commit history (`3c6c1dd`, `2922986`, `f84e4cd`) shows these were iteratively-discovered correctness fixes, not upfront design.

---

## Likely interviewer questions & strong answers

**1. Why a *reverse* BFS, and why exclude `IMPORTS` edges?**
> The question is "if this function changes, what's affected?" — that's the set of things that depend on it, i.e. its callers, transitively. So I walk edges backwards: `A calls B` means if `B` changed, `A` is at risk. I follow CALLS, CONTAINS, EXTENDS, IMPLEMENTS in reverse but **deliberately skip IMPORTS** ([query.go:20-22](../internal/graph/query.go:20)) — import edges are far too coarse; every file importing a package would look affected and the radius would explode. Depth is capped (default 5) so impact stays local and the report stays actionable.

**2. The diff gives you line numbers in the *new* file; the graph was built from some indexed snapshot. How do you map between them, and what breaks?**
> Two sub-problems. First, *line accounting*: I parse the unified diff and track new-file coordinates carefully — added lines advance the counter, deleted lines don't ([diff/parser.go:92-97](../internal/diff/parser.go:92)) — then test interval overlap against each function's line span. Second, *path matching*: diff paths and graph paths rarely match exactly (repo-relative vs absolute vs module-rooted), so I do segment-aware bidirectional suffix matching with a longest-match, alphabetical-tiebreak rule ([diff_mapper.go:111-142](../internal/analyzer/diff_mapper.go:111)) — segment-aware so `utils/helper.go` doesn't falsely match `myutils/helper.go`. The honest failure mode: this assumes the graph was indexed at the *same revision* as the diff's base. If the graph is stale, line numbers drift and mapping degrades. That's a real limitation (see weaknesses).

**3. You have SCIP *and* your own parser. Why both? When does each win?**
> SCIP is compiler-grade — accurate cross-file and cross-package resolution — so every edge is `EXACT`. But it requires an external indexer to be installed and to succeed, and for non-Go languages that's flaky. The parser fallback is always available and in-process, but cross-file resolution is heuristic. So: SCIP when you can get it (the whole Go path is built on it), parser when you can't. They produce the identical model, and `validate-scip` lets me *measure* how close the cheap one gets to the precise one — on `gin` they share ~3,460 edges with SCIP finding ~2,200 more, mostly IMPLEMENTS and method containment the heuristics miss.

**4. Cross-file call resolution is fundamentally imprecise. How do you handle that honestly?**
> I never emit a wrong edge to look complete. When resolution is ambiguous — multiple candidates — I emit **no edge** and record it as an ambiguous `ResolutionIssue` ([builder.go](../internal/graph/builder.go)). When I do resolve by a weaker signal (a globally-unique name rather than a static type), I tag the edge `HEURISTIC`. That confidence then **propagates as the weakest link** through the blast radius and **penalizes the review score**. So the output distinguishes "this is definitely affected" from "this might be affected, based on a guess." Precision over recall, surfaced to the user.

**5. Why two different scores — a risk *label* and a review *score*?**
> They answer different questions. The label (HIGH/MEDIUM/LOW) is for humans and the CI gate — it's a threshold ladder on production caller count, export status, and depth ([risk_scorer.go:40-69](../internal/analyzer/risk_scorer.go:40)). The review score is purely for *ordering* the report ([blast_radius.go:242](../internal/analyzer/blast_radius.go:242)) — it's continuous, with capped caller bonuses so one mega-hub can't dominate, plus depth and confidence adjustments. Mixing "is this dangerous?" with "show this first" into one number would make both worse.

**6. Why is this better than just running the test suite, or an LLM reviewer like CodeRabbit?**
> It's complementary, not a replacement — the README is explicit about that. Tests tell you what broke *if you have coverage*; TraceScope tells you what *could* break so a reviewer knows where to look, including untested paths. Versus an LLM reviewer: this is deterministic, fast, runs offline, and is grounded in an actual call graph rather than a probabilistic read of the diff — so it won't hallucinate an impact, and its exit code can hard-gate CI. The honest framing is "a focused, deterministic impact lens," not "an AI reviewer."

**7. Why so much effort on deterministic output?**
> Because it runs in CI and posts a comment that gets *updated* in place. If the affected-function ordering flapped between runs due to Go's randomized map iteration, every push would churn the comment and erode trust. So map iterations that produce edges are sorted ([scip.go:221-227](../internal/graph/scip.go:221)), and every ranked list has a unique final tiebreaker — node ID for affected functions ([blast_radius.go:199](../internal/analyzer/blast_radius.go:199)). Determinism is a correctness requirement, not a nicety.

**8. How does incremental indexing work, and when is the cache invalidated?**
> On the parser path, each file's `FileResult` is cached keyed by path, with the file's **SHA-256 content hash** as the freshness key ([index.go:126-132](../internal/cmd/index.go:126)). On re-index I re-hash each file; if it matches `FileMetadata.Hash` I reuse the cached parse, otherwise I re-parse just that file, then rebuild the whole graph from the merged results so it's always complete. Deleted files are pruned from the cache. The SCIP path uses a *different* strategy — mtime comparison against sources and markers ([scipIndexCacheFresh](../internal/cmd/index.go)). I'd flag that the mtime approach can be fooled by `git checkout` rewinding timestamps.

**9. What's the complexity and the scaling ceiling of the blast-radius computation?**
> The BFS itself is O(V + E) over the reachable subgraph, bounded by maxDepth. The catch is I rebuild the reverse adjacency list from the flat edge slice on every call ([query.go:41-65](../internal/graph/query.go:41)) — O(E) setup per query — because the graph is stored as two flat slices for easy JSON serialization, not as a persistent adjacency structure. For a CLI one-shot that's fine; the whole `gin` graph (1,500 nodes / 5,700 edges) builds in seconds. The ceiling is that the entire graph loads into memory and adjacency is rebuilt per query, so a very large monorepo would want a persistent indexed store. I consciously chose the slice-backed queue over `container/list` in this hot path to cut allocations ([query.go:76-85](../internal/graph/query.go:76)).

**10. How would you add a new language?**
> Implement the `LanguageParser` interface — `Parse(path, source) (*FileResult, error)` and `Language()` — and register it in the `Registry` ([registry.go](../internal/parser/registry.go)); the file walker already maps extensions to languages. Everything downstream (graph build, traversal, risk, output) is language-agnostic because it operates on the shared model. For *good* results you also want a SCIP indexer for that language, since the parser path's cross-file resolution is heuristic. That's exactly why TS/Python are "experimental" — the plumbing is there, but their resolution quality isn't trustworthy yet.

**11. You shell out to `git`, `gh`, `node`, and SCIP binaries, and post to GitHub. What are the security considerations?**
> All subprocess calls pass arguments as separate argv, never through a shell, so there's no shell-injection surface ([git_log.go](../internal/ownership/git_log.go), [github.go](../internal/output/github.go)), and each has a context timeout. The web tier keeps the GitHub OAuth token **server-side** — the Next.js route fetches it from better-auth's linked-account store and proxies GitHub calls so the token never reaches the browser ([api/github/route.ts](../web/app/api/github/route.ts)). Things I'd call out as weaker: the local server has no auth and shells `git diff` with query-param refs that aren't validated as refs (argument-injection class, low impact since it's localhost-dev-only), and the HTML report interpolates node names via `innerHTML` without escaping — fine for self-generated repos, but I'd escape it before trusting arbitrary input.

**12. How do you know the graph is *correct*? How do you test a static-analysis tool?**
> Three layers. Unit tests per package — 33 `_test.go` files, including dedicated inheritance-resolution tests and `review_fixes_test.go` files that pin specific bugs found in review so they can't regress. The `validate-scip` differential harness cross-checks the two independent backends against each other. And a real-repo benchmark on `gin` ([docs/benchmark-real-repo.md](benchmark-real-repo.md)) that's meant to be re-run after mapper changes to catch graph-shape and performance regressions. Tests run with `-race` because of the worker-pool concurrency.

---

## Honest weaknesses & how I'd improve them

Interviewers reward candidates who know the limits. These are real, from the code.

| Weakness (where) | Why it matters | How I'd improve it |
|---|---|---|
| **Graph/diff revision skew** ([diff_mapper.go](../internal/analyzer/diff_mapper.go)) | Mapping assumes the graph was indexed at the diff's base revision; a stale graph drifts line numbers and silently mis-maps. | Stamp the graph with the commit it was built from; detect mismatch and either re-index changed files (the hash machinery already exists) or re-anchor line numbers through the diff itself. |
| **TS/Python are heuristic-only and "experimental"** (README, [typescript.go](../internal/parser/typescript.go)) | Blast radius on those languages isn't trustworthy; diff-to-function mapping is unreliable. | Lean on SCIP indexers for TS/Python (the same path that makes Go reliable) instead of the heuristic parser, validate coverage with `validate-scip`, and gate non-Go output behind a confidence floor. |
| **"Ownership" is `git log`, not `git blame`** ([git_log.go:76](../internal/ownership/git_log.go:76)) | Attributes a whole file to its last committer — a one-line formatting commit can mislabel the owner. | Use `git blame` weighted by lines-in-the-changed-hunks, or commit-frequency over a window, for a truer owner. |
| **In-memory whole-graph + per-query adjacency rebuild** ([query.go:41](../internal/graph/query.go:41)) | Won't scale to very large monorepos; O(E) setup per query. | Persist a precomputed adjacency / use an embedded graph store (e.g. an indexed KV or SQLite) and keep the BFS on a memory-mapped view. |
| **Backend endpoints with no UI** ([server.go](../internal/server/server.go)) | `/api/why`, `/api/analyze/branches`, and `/api/reload` are implemented server-side but unused by the dashboard. (The previously-noted no-op WebSocket and unused web deps have since been removed.) | Finish the why / branch-diff UI or trim the endpoints; keep the dashboard scoped as the demo-only surface it's documented to be. |
| **Atomic write isn't power-loss-safe** ([store.go:20-47](../internal/graph/store.go:20)) | Temp-file+rename survives a process crash, not an OS/disk crash (no `fsync`). | `fsync` the temp file before rename; add a schema-version field for forward migration. |
| **PR-comment lookup scans only the first API page** ([github.go](../internal/output/github.go)) | On a PR with many comments the marker could be missed, producing a duplicate. | Paginate the `issues/comments` query, or use the GraphQL minimized-comment API. |
| **Magic-number scoring weights** ([blast_radius.go:242-280](../internal/analyzer/blast_radius.go:242)) | The review-score constants aren't validated against labeled outcomes; only three risk thresholds are configurable. | Expose the weights in config, and calibrate them against a labeled set of "this change actually caused a regression" PRs. |
| **mtime-based SCIP cache freshness** ([index.go](../internal/cmd/index.go)) | `git checkout`/clock skew can mark a stale index "fresh." | Switch the SCIP cache to content-hash freshness like the parser cache already uses. |
| **HTML report `innerHTML` with unescaped names** ([report_template.html](../internal/output/report_template.html)) | DOM-XSS if a symbol name contains HTML (the Go side only sanitizes the `<script>` JSON block). | Escape interpolated names or build nodes with `textContent`. |

**Framing tip:** when asked "what would you do differently," lead with the **revision-skew** and **scoring-calibration** items — they show you understand that the tool's *value* depends on the graph being current and the risk model being validated, not just on the code compiling.
