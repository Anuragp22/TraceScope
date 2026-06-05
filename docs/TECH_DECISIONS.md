# TraceScope — Technology & Design Decisions

> For each item: **what it is**, **how this project uses it** (with file references), and **likely reason chosen over alternatives**.
> "Likely reason" is inference unless the code/comments state it; inferred rationale is marked **[ASSUMED]**. Anything the code does *not* justify is flagged explicitly.

---

## A. Language & runtime

### Go (module `github.com/anurag/tracescope`, [go.mod](../go.mod))
- **What:** The entire CLI, graph engine, parsers, analyzers, output, and server are Go.
- **Version note:** [go.mod:3](../go.mod:3) declares `go 1.25.0`; the README says "Go 1.22+"; CI pins `1.22` ([tracescope.yml:21](../.github/workflows/tracescope.yml:21)). Mild inconsistency worth knowing.
- **Likely reason over alternatives [ASSUMED]:** Go is the natural choice because (1) the target language is Go itself, so the project can use the **standard-library `go/ast`/`go/types`** to parse and type-check Go for free (no external toolchain), (2) single static binary distribution suits a CI tool, (3) cheap goroutine concurrency for the parser/git worker pools, and (4) the official SCIP Go bindings exist. A Python/Node tool would have had to shell out for Go parsing instead of using a first-class AST library.

---

## B. Go third-party libraries (direct dependencies)

### `github.com/spf13/cobra` v1.8.0 — CLI framework
- **What:** Command/subcommand framework with flags, help, and lifecycle hooks.
- **Use:** The whole command tree — `index`, `analyze`, `why`, `hotspots`, `report`, `serve`, `validate-scip` ([internal/cmd](../internal/cmd)). Root `PersistentPreRun` loads config before any subcommand ([root.go:21-39](../internal/cmd/root.go:21)). Every command sets `SilenceErrors`/`SilenceUsage` so errors funnel to one exit mapper.
- **Likely reason over alternatives:** Cobra is the de-facto Go CLI standard (kubectl, gh, hugo). vs stdlib `flag`: Cobra gives nested subcommands, persistent flags, and the `Flags().Changed(name)` API used to implement *config-vs-flag precedence* ([analyze.go:50](../internal/cmd/analyze.go:50)) — hard to do cleanly with `flag`. **[ASSUMED]** but strongly idiomatic.

### `github.com/scip-code/scip/bindings/go/scip` v0.7.0 + `google.golang.org/protobuf` v1.36.11 — SCIP index ingestion
- **What:** Protobuf bindings for [SCIP](GLOSSARY.md) (SCIP = "SCIP Code Intelligence Protocol", Sourcegraph's successor to LSIF). `protobuf` decodes the `.scip` binary.
- **Use:** [scip.go](../internal/graph/scip.go) unmarshals indexes (`proto.Unmarshal`), reads occurrences/symbols/relationships, and uses helpers like `scip.IsGlobalSymbol`. [index.go](../internal/cmd/index.go) also rewrites SCIP document paths for monorepos.
- **Likely reason over alternatives:** SCIP is the *point* of the project — it's the source of compiler-grade, cross-file-accurate edges that the heuristic parser can't match. Alternatives: LSIF (older, graph-encoded, harder to consume), or building a full type-resolver per language (enormous). Reusing the indexer ecosystem (`scip-go`, `scip-typescript`, `scip-python`) offloads the hardest part.

### `github.com/smacker/go-tree-sitter` v0.0.0-2024… — multi-language parsing fallback
- **What:** CGo bindings to [tree-sitter](GLOSSARY.md) incremental parsers, with grammars for JS/TS/TSX/Python.
- **Use:** The parser *fallback* for non-Go languages — [javascript.go](../internal/parser/javascript.go), [typescript.go](../internal/parser/typescript.go), [python.go](../internal/parser/python.go) do hand-written recursive walks over the tree-sitter CST; [treesitter.go](../internal/parser/treesitter.go) wraps parsing in a 30s timeout.
- **Likely reason over alternatives:** Tree-sitter gives one uniform parsing API across many languages without writing a parser per language. vs language-native parsers (Babel for JS, Python's `ast`): those would require shelling to Node/Python at parse time for *every* language; tree-sitter keeps parsing in-process. The tradeoff is that tree-sitter is **syntactic only** — it can't resolve types across files, which is exactly why these languages stay "experimental" and SCIP is preferred. Note: Go does **not** use tree-sitter; it uses `go/ast` (see §C).

### `github.com/sourcegraph/go-diff` v0.7.0 — unified diff parsing
- **What:** Parses unified/git diffs into structured `FileDiff`/`Hunk` objects.
- **Use:** [diff/parser.go:25](../internal/diff/parser.go:25) uses `diff.ParseMultiFileDiff` for the *structure* (file splits, hunk headers), then **hand-rolls the hunk-body line accounting** itself (added lines advance the new-file counter, deleted lines don't — [parser.go:60-115](../internal/diff/parser.go:60)).
- **Likely reason over alternatives:** Parsing the multi-file diff envelope and hunk headers correctly is fiddly; the library handles it. But the project doesn't trust the library for the precise new-file line ranges (which it needs to map onto function line spans), so it computes those by walking the body — a "use the library for the boring part, control the part that matters" decision. vs `git` plumbing (`git diff --numstat`): the tool reads diffs from **stdin**, so it must parse the diff text itself, not re-run git.

### `github.com/bmatcuk/doublestar/v4` v4.10.0 — `**` glob matching
- **What:** Glob library supporting `**` (recursive) patterns, which stdlib `filepath.Match` cannot do.
- **Use:** Ignore-pattern filtering ([helpers.go](../internal/cmd/helpers.go), [hotspots.go:133](../internal/analyzer/hotspots.go:133)) and CODEOWNERS matching ([codeowners.go:128-163](../internal/ownership/codeowners.go:128)).
- **Likely reason over alternatives:** `.gitignore`/CODEOWNERS-style patterns (`vendor/**`, `**/*.go`) require `**`, which `path/filepath.Match` doesn't support. Writing a correct `**` matcher by hand is error-prone, so a focused library is the right call.

### `github.com/fatih/color` v1.18.0 — terminal styling
- **What:** ANSI color/style with automatic TTY detection (strips codes when piped).
- **Use:** All human-readable terminal output — [terminal.go:16-23](../internal/output/terminal.go:16), [output/hotspots.go](../internal/output/hotspots.go), [output/path.go](../internal/output/path.go).
- **Likely reason over alternatives:** Auto-TTY-detection means colors disappear automatically under `| jq` or CI logs without a manual `isatty` check. vs raw ANSI escape strings: the library handles Windows console + non-TTY correctly (pulls in `go-colorable`/`go-isatty` as indirect deps).

### `github.com/rs/zerolog` v1.32.0 — structured logging
- **What:** Zero-allocation structured/leveled logger.
- **Use:** Diagnostic/debug logging across `cmd`, `parser`, `server` (e.g. [root.go](../internal/cmd/root.go) wires a `ConsoleWriter` to stderr; `-v` flips to debug level).
- **Likely reason over alternatives [ASSUMED]:** Structured leveled logging that doesn't pollute stdout (logs go to stderr, preserving the stdout-is-JSON contract). vs stdlib `log`: zerolog gives levels and structured fields; vs `logrus`: zerolog is faster/lower-alloc. Either would have worked; this is a reasonable default.

### `github.com/gorilla/mux` v1.8.1 — HTTP router
- **What:** HTTP request router with method/path matching.
- **Use:** [server.go:67-88](../internal/server/server.go:67) registers all `/api/*` routes with method constraints.
- **Likely reason over alternatives [ASSUMED]:** Mux gives method-scoped routes (`GET`/`POST`) and path vars with minimal ceremony. vs stdlib `net/http.ServeMux`: at the time the code targets, stdlib mux lacked method-based routing (added in Go 1.22). vs chi/echo/gin: mux is the lightest mature option for a handful of routes. Reasonable, not load-bearing.

### `github.com/rs/cors` v1.11.1 — CORS middleware
- **What:** Configurable CORS handler.
- **Use:** [server.go:80-87](../internal/server/server.go:80) — hardcoded allowlist of `localhost:3000`/`3001` for the dev dashboard.
- **Likely reason over alternatives:** The Next.js dev server runs on a different origin (`:3000`) than the Go API (`:4000`), so the browser needs CORS. A focused middleware is cleaner than hand-rolling preflight handling.

### `gopkg.in/yaml.v3` v3.0.1 — config parsing
- **What:** YAML unmarshaller.
- **Use:** [config.go](../internal/config/config.go) parses `.tracescope.yaml` into `Config`.
- **Likely reason over alternatives:** YAML is the conventional format for dev-tool config (think `.golangci.yml`, GitHub Actions). vs JSON: YAML allows comments and is friendlier to hand-edit; vs TOML: YAML is more common in this ecosystem. **[ASSUMED]** but idiomatic.

---

## C. Go standard library used in non-obvious ways

These are "decisions" as much as the third-party deps — the project leans on stdlib for the hard parts.

### `go/parser` + `go/ast` + `go/types` + `go/importer` — native Go analysis
- **Use:** [golang.go](../internal/parser/golang.go) parses Go with `goparser.ParseFile(..., AllErrors)` and **runs the full type checker** (`cfg.Check` with `importer.Default()`) to recover *static receiver types* — so it knows `user.Save()` resolves to `models.User.Save`, not just "a call named Save" ([golang.go:212-224](../internal/parser/golang.go:212)). Type errors are swallowed (`Error: func(error){}`) so partial/incomplete code still yields data.
- **Also:** [scip.go:257-287](../internal/graph/scip.go:257) re-runs this Go parser to fix SCIP's identifier-only function ranges (see [INTERVIEW_PREP.md](INTERVIEW_PREP.md) hardest-problem section).
- **Why it matters:** This is the difference between Go being "the reliable language" and TS/Python being "experimental." Go gets compiler-grade typing essentially for free; the others can't.

### `html/template` + `template.JS` + `embed` — the HTML report
- **Use:** [report.go](../internal/output/report.go) embeds `d3.v7.min.js` and the HTML template at compile time (`go:embed`), injects graph+analysis JSON as `template.JS`, and manually sanitizes `</` → `<\/` to prevent `</script>` breakout ([report.go:54-56](../internal/output/report.go:54)).
- **Why:** `go:embed` makes the report a single self-contained file with zero CDN/network dependency — it opens offline. The `template.JS` + manual sanitization is a deliberate, two-layer trust decision (opt out of auto-escaping for the JS payload, then re-add a targeted defense).

### `container/list` vs slice-backed queue — two BFS implementations
- **Use:** [pathfinder.go](../internal/graph/pathfinder.go) (the `why` shortest-path) uses `container/list`; [query.go:82-98](../internal/graph/query.go:82) (blast radius) uses a **slice + head index** "to avoid per-element list allocations."
- **Why:** Blast radius runs on every PR and over potentially large graphs, so allocation matters; `why` is an interactive one-shot where clarity wins. A conscious perf-vs-readability split.

### `crypto/sha256` — content-addressed incremental indexing
- **Use:** [registry.go](../internal/parser/registry.go) hashes each file's bytes; [index.go:126-132](../internal/cmd/index.go:126) compares the hash to `FileMetadata.Hash` to skip re-parsing unchanged files. Node IDs are also sha256-derived ([builder.go](../internal/graph/builder.go)).
- **Why:** Content hashing is a robust freshness key (immune to mtime games). Note the SCIP cache uses **mtime** comparison instead ([index.go](../internal/cmd/index.go)) — two different staleness strategies coexist.

### `os/exec` + `context` (timeouts) — shelling out safely
- **Use:** External tools are invoked, never via a shell: SCIP indexers ([index.go](../internal/cmd/index.go)), `gh` for PR comments ([github.go](../internal/output/github.go)), `git log` for authorship with a per-file 5s timeout ([git_log.go:72-73](../internal/ownership/git_log.go:72)), `git diff` server-side ([server.go](../internal/server/server.go)).
- **Why:** Passing args as separate argv (not a shell string) avoids shell injection; `context` timeouts prevent a hung subprocess from stalling the run.

### `sync` worker pools — bounded concurrency
- **Use:** Parsing ([registry.go](../internal/parser/registry.go), pool sized `min(NumCPU, 8, jobs)`) and git author lookup ([git_log.go](../internal/ownership/git_log.go), pool of 8) both use buffered-channel + `WaitGroup` + `Mutex` fan-out.
- **Why:** Parse/git work is embarrassingly parallel and I/O-bound; bounding the pool prevents spawning thousands of goroutines/processes on a big repo.

---

## D. Architectural patterns (not libraries)

### Single exit authority + typed risk error
- `main()` is the only `os.Exit` ([main.go](../cmd/tracescope/main.go)); `exit.go` is the only error→code mapper. Risk results ride up as a typed `*analyzer.RiskExitError` and are unwrapped with `errors.As` ([exit.go:22-31](../internal/cmd/exit.go:22)).
- **Why:** Keeps commands pure/testable (they return `error`, never exit), and keeps the CI contract (1/2 = risk, 3 = failure) in one auditable place.

### Dependency injection via package-level `var` function values
- `scipLookPath` and `runSCIPCommand` are package-level `var`s holding functions ([index.go:205-213](../internal/cmd/index.go:205)), so tests can swap them without an interface or a mock framework.
- **Why:** Lightweight seam for testing subprocess calls — idiomatic Go "monkeypatch via var."

### Strategy + registry for parsers
- `LanguageParser` interface ([parser.go:55](../internal/parser/parser.go:55)); `Registry` maps `Language → parser` and dispatches ([registry.go](../internal/parser/registry.go)).
- **Why:** Adding a language = implement one interface + register it; the rest of the pipeline is language-agnostic.

### Confidence-aware edges + self-auditing graph
- Every edge carries `EXACT`/`HEURISTIC`; `ResolutionStats`/`ResolutionIssues` record what failed ([types.go](../internal/graph/types.go)).
- **Why:** Static analysis is inherently imprecise across files; rather than hide it, the model quantifies and surfaces it (and penalizes heuristic edges in ranking).

### Content-signature graph comparison
- [compare.go](../internal/graph/compare.go) compares two graphs by `Type:relpath:Name` signatures (not IDs, which are hashes that differ between backends), with a special case to reconcile Go package-vs-file import representations.
- **Why:** Enables the `validate-scip` differential harness — a built-in way to measure the cheap backend against the precise one.

### Atomic write (temp file + rename)
- [store.go:20-47](../internal/graph/store.go:20) and [cache.go](../internal/parser/cache.go) write to a temp file in the same directory, then `os.Rename`.
- **Why:** A crash mid-write can't corrupt the existing graph/cache (rename is atomic on a single filesystem). **Caveat:** no `fsync`, so it's crash-safe but not power-loss-safe.

### Copy-on-replace snapshotting (server)
- [server.go](../internal/server/server.go) reads via `graphSnapshot()` under `RLock`, the sole writer swaps the pointer under `Lock`.
- **Why:** In-flight HTTP readers keep a consistent snapshot while `/api/reload` swaps the graph.

---

## E. Web stack ([web/package.json](../web/package.json))

| Library | What it does | Likely reason chosen / over what |
|---|---|---|
| **Next.js 16** (App Router) | React framework: SSR shell, client pages, API routes | One framework for UI *and* the server-side GitHub OAuth proxy. vs plain React+Vite: Next's API routes let the OAuth token stay server-side. **[ASSUMED]** |
| **React 19** + **react-dom** | UI runtime | Default for Next. |
| **@tanstack/react-query 5** | Server-state/data fetching (caching, retries, stale time) | Declarative data layer for all `/api` calls; avoids hand-rolled fetch/loading state. vs SWR: equivalent; TanStack is feature-richer. **[ASSUMED]** |
| **better-auth 1.5** | Auth framework: GitHub OAuth, sessions, linked-account token vault | Enables fetching the user's GitHub token **server-side** to pull PR diffs without exposing it to the browser ([api/github/route.ts](../web/app/api/github/route.ts)). vs NextAuth: better-auth's `getAccessToken` for linked accounts is the key feature used. **Caveat:** no DB adapter configured → in-memory sessions (dev-only). |
| **react-force-graph-3d** + **three** | 3D force-directed graph viz | The Explore page's drill-down graph ([explore/page.tsx](../web/app/explore/page.tsx)); dynamically imported `ssr:false` (needs `window`). vs 2D/D3: a higher-impact visual. The Go HTML report separately uses **D3 v7** for a 2D version. |
| **Tailwind CSS v4** + **shadcn tokens** + **tw-animate-css** | Utility CSS + design tokens | Fast styling without a component library lock-in; all UI is hand-rolled Tailwind. |
| **lucide-react** | Icon set | Lightweight icons across nav/pages. |

**Honest completeness note:** The dashboard is functional, not a mockup. The remaining gap is that three backend endpoints (`/api/why`, `/api/analyze/branches`, `/api/reload`) are implemented server-side but have no dashboard UI yet. The README itself scopes the dashboard as demo-only. (Earlier-unused code — the `@xyflow/react` dependency, a duplicate `lib/github.ts` client, scaffolded shadcn `ui/*` stubs, and a no-op WebSocket — was removed during cleanup.)

---

## F. Build, release & CI tooling

- **Makefile / goreleaser** ([.goreleaser.yaml](../.goreleaser.yaml)) — cross-platform release builds. **[ASSUMED]** standard Go release tooling.
- **GitHub Actions** ([.github/workflows](../.github/workflows)) — `ci.yml` (build/test), `release.yml`, and `tracescope.yml` which **dogfoods the tool** by running blast-radius analysis on every PR.
- **`-race -count=1`** test invocation (README §Testing) — race detection is on, appropriate given the worker-pool concurrency.
