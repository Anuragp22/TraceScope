# Code Review Fixes — Plain-English Explanation

_Date: 2026-05-21. This document explains 11 bugs that were found in a full
code review of TraceScope and how each was fixed. It is written to be readable
without deep knowledge of the codebase — keep it as a study reference._

---

## 1. Refresher: what does TraceScope actually do?

You give TraceScope a **diff** (the set of changes in a pull request). It tells
you **what else in the codebase might break** because of those changes, ranked
by risk, so a reviewer knows where to look.

It works in three steps:

1. **Index** — read every source file and build a *dependency graph* of the
   whole repo.
2. **Map the diff** — figure out which functions the PR actually changed.
3. **Blast radius** — starting from the changed functions, walk the graph
   outward to find everything that depends on them, score the risk, and print
   a report (or post a GitHub PR comment).

### Key terms you need

| Term | Meaning |
|------|---------|
| **Graph** | A model of the codebase as dots and arrows. |
| **Node** | A dot in the graph — a file, a function, or a class. |
| **Edge** | An arrow between two nodes — e.g. "function A *calls* function B", or "file *contains* function". |
| **Blast radius** | Everything reachable by following arrows *backwards* from a changed function (i.e. everything that depends on it). |
| **SCIP** | "Source Code Intelligence Protocol" — a precise index of a codebase produced by external tools (`scip-go` for Go, `scip-typescript` for TypeScript). TraceScope reads SCIP files to build an accurate graph. |
| **Parser fallback** | If SCIP isn't available, TraceScope parses the code itself. Less precise. |

---

## 2. How the bugs were found

Five automated reviewers each read one part of the codebase looking for real
defects. They found 18 issues. We fixed the 11 most important ones (the ones
that produce *wrong results*, not just cosmetic problems).

**Every fix has a test.** A test is a small piece of code that proves the bug
existed and proves the fix works. For each bug we first wrote a test that
*failed*, then changed the code until it *passed*. The new tests live in files
named `review_fixes_test.go`.

After all fixes: the full test suite passes (9 packages) and `go vet` (Go's
built-in problem checker) is clean.

---

## 3. The 11 bugs, in plain English

### Bug 1 — The graph blamed the wrong function for a call

**What was wrong:** When TraceScope recorded "this line calls function X", it
guessed *which function the line belongs to* by finding the nearest function
definition above it. It never checked whether the line was actually *inside*
that function's body. So a line of code sitting *between* two functions could
be wrongly attributed to the function above it.

**Why it mattered:** The graph is the foundation of everything. A wrong arrow
means the blast radius can point at the wrong code.

**What changed:** SCIP can optionally tell us the *full body range* of a
function (its start AND end line). We now use that when it's available, so a
line outside a function's body is correctly attributed to the file instead.

**The catch (important — see section 4):** The Go indexer (`scip-go`) does not
provide body ranges, only the line of the function's name. So this fix fully
works for **TypeScript** but **Go still uses the old guess**. That's a
limitation of the external tool, not of TraceScope.

_File: `internal/graph/scip.go`_

### Bug 2 — Deleting a function showed "no impact"

**What was wrong:** If a PR *deleted* a function, TraceScope reported zero
affected code.

**Why it mattered:** Deleting a function is one of the most dangerous changes
possible — every caller of it breaks. TraceScope was silent exactly when it
mattered most.

**What changed:** A deleted file is now treated like a new file — all of its
functions are fed into the blast-radius analysis, so their callers get found.

_File: `internal/analyzer/diff_mapper.go`_

### Bug 3 — Errors were invisible, and a broken run looked "high risk"

**What was wrong:** When TraceScope hit an error (bad input, missing index,
etc.), it printed **nothing at all** and exited with code `1`. But code `1`
also means "high-risk PR". So in CI, a *broken tool* was indistinguishable from
a *risky pull request*.

(An "exit code" is a number a program returns when it finishes. `0` = success;
other numbers signal different outcomes. CI systems read this number.)

**Why it mattered:** A user would see a blank screen with no explanation, and
CI would treat a misconfiguration as a dangerous PR.

**What changed:** Errors now print a clear message (`Error: ...`) and exit with
code `3` — a code that does not collide with the risk codes (`0`/`1`/`2`).

_Files: `cmd/tracescope/main.go`, new `internal/cmd/exit.go`, README exit-code table_

### Bug 4 — The "hotspots" list hid the most important functions

**What was wrong:** "Hotspots" ranks functions by how connected they are. The
score was `callers × callees`. A function called by 100 others but calling
nothing scored `100 × 0 = 0` — dead last.

**Why it mattered:** A heavily-used function with no outgoing calls (a shared
utility, an API endpoint) is exactly what you want flagged. It was being
buried.

**What changed:** The score is now `callers × 2 + callees`, so heavily-called
functions rank high.

_File: `internal/analyzer/hotspots.go`_

### Bug 5 — "Implements" relationships were never recorded

**What was wrong:** The graph has two kinds of inheritance arrow: EXTENDS (a
type builds on another type) and IMPLEMENTS (a type fulfils an interface). Due
to a copy-paste mistake, every relationship was labelled EXTENDS — IMPLEMENTS
was never produced.

**What changed:** When the parent is an interface, the arrow is now correctly
labelled IMPLEMENTS.

_File: `internal/graph/builder.go`_

### Bug 6 — `why` silently picked a random answer when ambiguous

**What was wrong:** The `why` command explains the call path between two
functions. If you typed a specific name like `pkg.Build` and two different
packages both had a `Build`, it silently picked the first one.

**Why it mattered:** You'd get a confident-looking but possibly wrong answer,
with no warning.

**What changed:** If a qualified name matches more than one function, it now
reports the ambiguity and shows the candidates instead of guessing.

_File: `internal/cmd/why.go`_

### Bug 7 — The same PR could produce different results each run

**What was wrong:** In several places the code looped over a Go *map*. Go
deliberately randomises map order. So edges and rankings could come out in a
different order on every run.

**Why it mattered:** For a CI tool, the same input must always give the same
output. Otherwise results look untrustworthy and diffs become noisy.

**What changed:** Those loops now use a sorted, stable order, and the final
ranking has a tie-breaker on a unique ID — so output is identical every run.

_Files: `internal/graph/scip.go`, `internal/analyzer/diff_mapper.go`,
`internal/analyzer/blast_radius.go`_

### Bug 8 — The file scanner could loop forever on Windows

**What was wrong:** When scanning a folder, the code used `filepath.Walk`,
which on Windows follows "directory junctions" (a kind of folder shortcut). A
junction pointing back at a parent folder would make the scan recurse forever.

**What changed:** Switched to `filepath.WalkDir` and added an explicit rule to
skip symlinks and junctions entirely.

_File: `internal/parser/walker.go`_

### Bug 9 — Anchored CODEOWNERS rules matched nothing

**What was wrong:** A `CODEOWNERS` file says who owns which files. A rule
anchored to the repo root, like `/internal/cmd/`, was supposed to own every
file under that folder — but the matching code treated it as a literal string
and matched nothing.

**Why it mattered:** TraceScope suggests reviewers from CODEOWNERS. Whole
folders silently had no owner.

**What changed:** Rewrote the pattern matcher so directory patterns correctly
own everything beneath them.

_File: `internal/ownership/codeowners.go`_

### Bug 10 — Duplicate "imports" arrows

**What was wrong:** When the parser built the graph, two import statements
pointing at the same file created two identical IMPORTS arrows.

**Why it mattered:** Duplicate arrows inflate counts and make graph
comparisons wrong.

**What changed:** Import arrows are now de-duplicated per file.

_File: `internal/graph/builder.go`_

### Bug 11 — A syntax error silently hid half a file

**What was wrong:** If a Go file had a syntax error, Go's parser still recovers
what it can. TraceScope kept the recovered half but **threw away the error** —
so it silently indexed an incomplete file as if it were complete.

**What changed:** The parse error is now surfaced (so it gets reported), *and*
the recovered part of the file is still kept (so we don't lose data either).

_Files: `internal/parser/golang.go`, `internal/parser/registry.go`_

---

## 4. The deeper finding (read this for interviews)

Bug 1 turned out to be the tip of something bigger.

To record a function in the graph, TraceScope needs to know **where the
function starts and ends** in the file. The Go indexer (`scip-go`) only
provides the **line of the function's name** — not where the body ends. We
verified this directly: 0 of 3,706 Go definitions carried body-range data.

This single gap causes **two** correctness problems for Go:

1. **Scope attribution** (Bug 1) — can't perfectly tell which function a line
   belongs to.
2. **Diff mapping** — to decide if a PR changed a function, TraceScope checks
   if the changed lines overlap the function's line range. With only the name
   line, a change deep inside a large function can be *missed*.

**The proper fix** is to compute Go function body end-lines ourselves — either
by scanning the source for the matching closing brace, or by reusing the
end-lines that TraceScope's own built-in Go parser already calculates
accurately.

This is the **top recommended next task**, and it is the **best interview
story** in this project: a genuinely hard, domain-specific correctness problem
("I found my code-intelligence index lacked function body ranges, which
silently weakened both scope attribution and diff mapping; here is how I'd
fix it").

---

## 5. How to check the work yourself

```bash
# Build
go build -o tracescope.exe ./cmd/tracescope

# Run every test (should all pass)
go test ./... -count=1

# See the new tests specifically
go test ./... -run ReviewFixes -v   # (test funcs live in *review_fixes_test.go)

# Prove Bug 3 is fixed — an empty diff now shows an error, not a blank screen
echo "" | ./tracescope.exe analyze     # prints "Error: empty diff", exits 3
```

---

## 6. One-line summary per file changed

| File | Bug(s) |
|------|--------|
| `internal/graph/scip.go` | 1, 7 |
| `internal/graph/builder.go` | 5, 10 |
| `internal/analyzer/diff_mapper.go` | 2, 7 |
| `internal/analyzer/hotspots.go` | 4 |
| `internal/analyzer/blast_radius.go` | 7 |
| `cmd/tracescope/main.go` + `internal/cmd/exit.go` | 3 |
| `internal/cmd/index.go`, `scip_validate.go` | 3 (stop double-printing errors) |
| `internal/cmd/why.go` | 6 |
| `internal/parser/walker.go` | 8 |
| `internal/parser/golang.go`, `registry.go` | 11 |
| `internal/ownership/codeowners.go` | 9 |
| `internal/*/review_fixes_test.go` | the new tests for all of the above |
