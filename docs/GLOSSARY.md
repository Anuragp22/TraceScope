# TraceScope — Glossary

Plain-English definitions for every domain term, piece of jargon, and naming convention in this codebase. Grouped by area. File references point to where the term is defined or used.

---

## Core domain concepts

| Term | One-line meaning |
|---|---|
| **Blast radius** | The set of functions that could be affected if a changed function changes — i.e. everything that (transitively) calls it. Computed in [query.go](../internal/graph/query.go). |
| **Dependency graph** | The in-memory/JSON model of a codebase as nodes (files, functions, classes) connected by edges (calls, contains, imports, inheritance). |
| **Diff → function mapping** | Turning a PR's changed *line ranges* into the *function nodes* whose source spans overlap them ([diff_mapper.go](../internal/analyzer/diff_mapper.go)). |
| **Why-path / impact path** | The chain of edges explaining *how* a changed symbol reaches an affected one, e.g. `Build -> registerReferenceEdges -> addEdge`. |
| **Hotspot** | A highly-coupled function (many callers and/or callees) flagged as structurally risky, independent of any diff ([hotspots.go](../internal/analyzer/hotspots.go)). |
| **Ownership** | Who should review a change — derived from `git log` last-author and the CODEOWNERS file ([internal/ownership](../internal/ownership)). |
| **Dogfooding** | The project runs *itself* on its own pull requests via [.github/workflows/tracescope.yml](../.github/workflows/tracescope.yml). |

---

## Graph model vocabulary ([internal/graph/types.go](../internal/graph/types.go))

| Term | One-line meaning |
|---|---|
| **Node** | A vertex in the graph: a file, a function, or a class. |
| **Edge** | A directed relationship between two nodes, with a type and a confidence. |
| **NodeType** | One of `file`, `function`, `class`. |
| **EdgeType — CONTAINS** | "File contains function" or "class contains method." |
| **EdgeType — CALLS** | "Function A calls function B." |
| **EdgeType — IMPORTS** | "File A imports file/package B." |
| **EdgeType — EXTENDS** | Inheritance/embedding (struct embeds struct, interface embeds interface). |
| **EdgeType — IMPLEMENTS** | A concrete type satisfies an interface. |
| **EdgeConfidence — EXACT** | The edge was resolved with full static/compiler information (always the case for SCIP edges). |
| **EdgeConfidence — HEURISTIC** | The edge was a best-guess from name/path matching, not provably correct. |
| **Seed (seed node)** | A starting node for the blast-radius traversal — a function the diff actually changed. |
| **Reverse BFS** | Breadth-first search that walks edges *backwards* (from changed code to its callers) to find what depends on it. |
| **Depth** | How many hops a node is from the nearest seed; the traversal is capped at `maxDepth` (default 5). |
| **Adjacency list** | The `target → [sources]` map rebuilt per query to traverse the graph efficiently. |
| **GraphData** | The whole serializable graph: nodes, edges, index source, stats, and diagnostics. |
| **IndexSource** | Which backend built the graph: `scip` (precise) or `parser` (fallback). |
| **Metadata** | Aggregate counts (files, functions, classes, edges, languages). |

---

## Confidence & resolution vocabulary

| Term | One-line meaning |
|---|---|
| **Resolution** | The act of figuring out which node a call/import/inheritance reference actually points to. |
| **Exact** (resolution) | Resolved unambiguously with static type/import info. |
| **Heuristic** (resolution) | Resolved by a fallback guess (e.g. a globally-unique name) and downgraded in trust. |
| **Ambiguous** | Multiple candidate targets existed, so the code emitted **no edge** rather than guess. |
| **Unresolved** | No candidate target could be found at all. |
| **ResolutionStats** | Counters of exact/heuristic/ambiguous/unresolved call and inheritance edges ([types.go:54](../internal/graph/types.go:54)). |
| **ResolutionIssue** | A single diagnostic record of an unresolved/ambiguous reference (capped at 200) so a reviewer can see where precision dropped. |
| **Confidence propagation / weakest link** | An affected node's confidence is the *least* confident hop along its path from a seed ([mergeConfidence, query.go:125](../internal/graph/query.go:125)). |

---

## SCIP vocabulary (the precise backend, [internal/graph/scip.go](../internal/graph/scip.go))

| Term | One-line meaning |
|---|---|
| **SCIP** | "SCIP Code Intelligence Protocol" — Sourcegraph's compiler-grade, protobuf-encoded code-index format (successor to LSIF). |
| **LSIF** | The older "Language Server Index Format" that SCIP replaces (referenced as background). |
| **SCIP indexer** | An external tool that emits a `.scip` index: `scip-go`, `scip-typescript`, `scip-python`. |
| **Symbol** | SCIP's globally-unique string identifier for a code entity (encodes package, type, member). |
| **Occurrence** | One appearance of a symbol at a source range, tagged with a role (definition / reference / import). |
| **Role** | A bitmask on an occurrence saying whether it's a Definition, Reference, Import, etc. |
| **Definition role** | The occurrence where a symbol is actually declared (used to create nodes). |
| **Reference role** | A use-site of a symbol (used to synthesize CALLS edges). |
| **Descriptor** | The structured suffix of a SCIP symbol string that names the entity (e.g. `Type#`, `method().`). |
| **Enclosing range** | The full body span of a definition (vs the identifier-only range); preferred so body-only diffs still overlap the node ([scip.go:176](../internal/graph/scip.go:176)). |
| **Enclosing symbol** | The parent symbol that contains another (e.g. the class owning a method); used for CONTAINS edges. |
| **Global vs local symbol** | Global = exported/cross-file (`scip.IsGlobalSymbol`); local = file-internal. |
| **Body-bounds refinement** | Re-running the native Go parser to fix SCIP's identifier-only function end-lines ([refineGoFunctionBounds, scip.go:257](../internal/graph/scip.go:257)). |
| **Document path rewriting** | Prefixing per-package SCIP document paths so a monorepo's indexes share one repo-relative namespace ([index.go:444](../internal/cmd/index.go:444)). |

---

## Risk & analysis vocabulary ([internal/analyzer](../internal/analyzer))

| Term | One-line meaning |
|---|---|
| **Risk level** | The human/CI label `HIGH`, `MEDIUM`, or `LOW` for an affected function. |
| **Review score** | An integer ranking signal (separate from risk level) used to *sort* affected functions ([blast_radius.go:242](../internal/analyzer/blast_radius.go:242)). |
| **Caller count (total)** | How many functions call this one, counting test callers. |
| **Production caller count** | Caller count *excluding* test callers — the number that drives the risk label ([risk_scorer.go:41](../internal/analyzer/risk_scorer.go:41)). |
| **Fan-in** | Number of inbound callers (how depended-upon a function is). |
| **Fan-out** | Number of outbound calls (how much a function depends on others). |
| **Coupling score** | Hotspot ranking metric = `inbound*2 + outbound` (inbound double-weighted so a heavily-called leaf isn't zeroed) ([hotspots.go:77](../internal/analyzer/hotspots.go:77)). |
| **Threshold (risk)** | Tunable caller cutoffs: `high_callers`=10, `high_exported_callers`=5, `medium_callers`=3 ([config.go](../internal/config/config.go)). |
| **RiskExitError** | A typed error carrying the risk-based process exit code (1 or 2) up to `main`. |
| **AffectedFunction** | One node in the blast radius, with its depth, risk, review score, confidence, and impact path. |
| **AnalysisResult** | The full output of an `analyze` run: changed files, changed/affected functions, stats. |

---

## Parsing vocabulary ([internal/parser](../internal/parser))

| Term | One-line meaning |
|---|---|
| **Parser fallback** | Building the graph from TraceScope's own parsers when no SCIP index is available. |
| **Tree-sitter** | An incremental parsing library producing a concrete syntax tree; used for JS/TS/Python. |
| **CST walk** | The hand-written recursive traversal of the tree-sitter syntax tree that extracts symbols. |
| **`go/ast` / `go/types`** | Go's standard-library AST and type-checker, used to parse Go natively (no tree-sitter). |
| **Receiver** | For a Go method `(u *User) Save()`, the `u *User` part; for a JS/Python call `a.b()`, the `a`. |
| **Receiver type** | The *static type* of a call's receiver (e.g. `User`), recovered by the Go type checker or TS inference. |
| **FileResult** | The uniform per-file parse output: functions, calls, imports, classes, content hash ([parser.go:42](../internal/parser/parser.go:42)). |
| **Class kind** | Whether a "class" node is a `struct`, `interface`, `type` alias, or `class`. |
| **Bases** | The parent types a class/struct/interface extends or implements. |
| **IsExport / IsTest / IsInit** | Node flags: exported/public symbol; defined in a test file; a Go `init()` function. |
| **Content hash** | SHA-256 of a file's bytes, used as the freshness key for incremental indexing. |
| **Parse cache** | `parse_cache.json` storing prior `FileResult`s so unchanged files aren't re-parsed. |
| **Worker pool** | Bounded set of goroutines (`min(NumCPU, 8, jobs)`) that parse files concurrently. |
| **Receiver type inference (TS)** | TraceScope infers a TS call's receiver type syntactically from `const x: Foo` / `new Foo()` declarations ([typescript.go](../internal/parser/typescript.go)); accurate cross-file typing comes from SCIP. |

---

## Diff vocabulary ([internal/diff](../internal/diff))

| Term | One-line meaning |
|---|---|
| **Unified diff** | The standard `git diff` text format (with `@@` hunk headers and `+`/`-` lines). |
| **Hunk** | One `@@ ... @@` block of contiguous changes within a file. |
| **Line range** | A start–end span of *new-file* line numbers that were added/changed in a hunk. |
| **New-file coordinates** | The discipline that added lines advance the line counter but deleted lines don't, so ranges map to the post-change file ([parser.go:92](../internal/diff/parser.go:92)). |
| **ChangedFile** | A parsed diff entry: path, its changed line ranges, and new/deleted flags. |

---

## Output & CLI/CI vocabulary ([internal/output](../internal/output), [internal/cmd](../internal/cmd))

| Term | One-line meaning |
|---|---|
| **Exit-code contract** | `0`=no significant risk, `1`=HIGH, `2`=MEDIUM, `3`=tool error — the interface CI gates on ([exit.go](../internal/cmd/exit.go)). |
| **Idempotent upsert** | Re-running posts an *update* to the existing PR comment, not a duplicate, keyed on a hidden marker. |
| **Comment marker** | The invisible `<!-- tracescope-blast-radius -->` HTML comment used to find the prior comment ([github.go:15](../internal/output/github.go:15)). |
| **`gh` CLI** | GitHub's official CLI, shelled out to read PRs and post/update comments. |
| **Reviewer focus** | The top-N highest-risk functions a reviewer should inspect first, with "inspect" hints. |
| **stdout-is-JSON convention** | Machine output goes to stdout; all human/log output goes to stderr, so piping to `jq` stays clean. |
| **`go:embed`** | Compile-time file embedding; bundles D3.js and the HTML template into the binary ([report.go:14](../internal/output/report.go:14)). |
| **Differential validation** | The `validate-scip` command comparing the SCIP graph vs the parser graph to catch regressions ([scip_validate.go](../internal/cmd/scip_validate.go)). |
| **Content signature** | A `Type:relativePath:Name` string used to compare nodes/edges across the two backends despite differing IDs ([compare.go](../internal/graph/compare.go)). |

---

## Codebase naming conventions

| Convention | Meaning |
|---|---|
| **`internal/<area>/`** | Go "internal" packages — importable only within this module; the whole engine lives here so it's not a public API. |
| **`run<Command>` (e.g. `runAnalyze`, `runIndex`)** | The Cobra command handler functions in [internal/cmd](../internal/cmd). |
| **`handle<Thing>` (e.g. `handleGraph`, `handleAnalyze`)** | The HTTP handlers in [server.go](../internal/server/server.go). |
| **`Print<X>` (e.g. `PrintAnalysis`, `PrintJSON`)** | The output renderers in [internal/output](../internal/output). |
| **`Build…` / `BuildFromSCIPFiles`** | Graph-construction entry points. |
| **`make<Type>ID` (`makeFileID`, `makeFuncID`, `makeClassID`)** | Helpers that derive a stable SHA-256-based node ID. |
| **`scip*` prefixed helpers** | Functions that parse the SCIP symbol/descriptor grammar ([scip.go](../internal/graph/scip.go)). |
| **snake_case JSON tags vs CamelCase Go fields** | Go structs use Go casing but serialize to `snake_case` (e.g. `is_export`, `affected_functions`) — the web/JSON contract. |
| **`*_test.go`** | Standard Go unit tests (33 of them). |
| **`review_fixes_test.go`** | Regression tests pinning specific bugs found in code review (e.g. commit `c9ab9c0` "Fix 11 correctness bugs"). |
| **`*_benchmark_test.go`** | Go benchmarks for graph build / blast radius. |
| **`*_inheritance_test.go`** | Tests focused on EXTENDS/IMPLEMENTS edge resolution. |

---

## File & directory conventions

| Path | Meaning |
|---|---|
| **`.tracescope/graph.json`** | The persisted dependency graph (the build artifact of `index`). |
| **`.tracescope/scip/*.scip`** | Generated per-language SCIP indexes, merged at build time. |
| **`.tracescope/parse_cache.json`** | Cached parser `FileResult`s for incremental indexing. |
| **`index.scip`** (repo root) | A pre-existing SCIP index that, if present, takes precedence over generating one. |
| **`.tracescope.yaml`** | Project config (ignore globs, max depth, format, risk thresholds), discovered by walking up from the cwd. |
| **`CODEOWNERS`** | GitHub's reviewer-assignment file, parsed from `./`, `.github/`, or `docs/` ([codeowners.go:33](../internal/ownership/codeowners.go:33)). |
| **`cmd/tracescope/`** | The `main` package / binary entry point. |
| **`web/`** | The optional Next.js dashboard. |
