# TraceScope

A CLI tool that parses codebases into dependency graphs and analyzes PR blast radius — helping teams understand the ripple effect of code changes before they ship.

## Features

- **Multi-language parsing** — Go (stdlib `go/ast`), JavaScript, TypeScript, Python (tree-sitter)
- **Dependency graph** — Functions, classes, files with CALLS, CONTAINS, IMPORTS, EXTENDS edges
- **Blast radius analysis** — Finds all functions affected by a code change
- **Risk scoring** — HIGH/MEDIUM/LOW based on caller count, depth, and export visibility
- **Hotspot detection** — Find the most coupled functions in your codebase
- **Call path tracing** — Show why function A depends on function B
- **Code ownership** — Git blame integration + CODEOWNERS parsing for reviewer suggestions
- **Incremental indexing** — Only re-parses changed files using content hashing
- **HTML report** — Interactive D3.js force-directed graph visualization
- **CI integration** — JSON output, risk-based exit codes, GitHub PR comments
- **Project config** — `.tracescope.yaml` for per-project settings

## Installation

### Prerequisites

- Go 1.21+
- GCC (64-bit) — required for tree-sitter CGo bindings
  - **Windows:** Install via [MSYS2](https://www.msys2.org/) → `pacman -S mingw-w64-ucrt-x86_64-gcc`
  - **macOS:** `xcode-select --install`
  - **Linux:** `apt install gcc` or equivalent

### Build

```bash
git clone https://github.com/Anuragp22/TraceScope.git
cd TraceScope
go build -o tracescope ./cmd/tracescope
```

## SCIP Indexing

TraceScope now prefers SCIP indexes and falls back to the built-in parser when SCIP is unavailable.

Install SCIP indexers on machines that run `tracescope index`:

```bash
go install github.com/sourcegraph/scip-go/cmd/scip-go@latest
npm install -g @sourcegraph/scip-typescript
npm install -g @sourcegraph/scip-python
```

Verify:

```bash
scip-go --version
scip-typescript --version
scip-python --version
```

Index selection behavior:
- If `index.scip` exists at repo root, TraceScope uses it directly.
- Otherwise, it tries `scip-go`, `scip-typescript index`, and `scip-python index`, then merges generated outputs from `.tracescope/scip/`.
- Nested JS/TS project roots are detected from the nearest ancestor `package.json`, so a repo like `web/package.json` is indexed as `scip-typescript@web`.
- Generated SCIP outputs are cached and reused while newer than their source files/project markers.
- On native Windows, `scip-python` is intentionally skipped because the published package fails on Windows path separators; use WSL/Linux CI for Python SCIP indexing.
- If no SCIP index is available, TraceScope falls back to parser-based indexing and `.tracescope/parse_cache.json`.

## Commands

### `tracescope index` — Build dependency graph

```bash
tracescope index .
tracescope index /path/to/project
```

Walks the directory, parses all supported files, builds the dependency graph, and saves it to `.tracescope/graph.json`. Supports **incremental indexing** — only re-parses files that changed since the last index.

```
TraceScope — indexing /path/to/project

  Found 46 files across 1 languages
  Parsed 3 files (43 cached, 0 errors)
  Built graph: 289 nodes, 644 edges

  Incremental: 3 files re-parsed, 43 cached
  Done in 24ms
```

### `tracescope analyze` — Blast radius analysis

```bash
git diff | tracescope analyze
git diff HEAD~3 | tracescope analyze --top 10
tracescope analyze --diff changes.patch --depth 3
tracescope analyze --ignore "vendor/**,dist/**"
tracescope analyze --format json
tracescope analyze --owners              # show code owners
tracescope analyze --github-comment      # post to PR
```

```
TraceScope — Blast Radius Analysis

  Changed Files (1):
    internal/graph/builder.go

  Changed Functions (1):
    Build (internal/graph/builder.go:26)

  Blast Radius (3 affected):

    MEDIUM RISK (2):
      TestBuilder_BasicGraph (builder_test.go:9) [0 callers, depth 1] by alice
      TestBuilder_ClassNodes (builder_test.go:59) [0 callers, depth 1] by bob

    LOW RISK (1):
      runIndex (cmd/index.go:27) [0 callers, depth 1] by alice

  Summary:
    Graph: 289 nodes, 644 edges
    Changed: 1 files, 1 functions
    Blast radius: 3 affected functions (depth 5)
    Risk: 0 high, 2 medium, 1 low

  Suggested Reviewers:
    @alice (2 files)
    @bob (1 file)
```

### `tracescope hotspots` — Find fragile code

```bash
tracescope hotspots              # top 10 most coupled functions
tracescope hotspots --top 20     # show more
tracescope hotspots --lang go    # filter by language
tracescope hotspots --format json
```

Ranks functions by **coupling score** (inbound callers x outbound calls):

```
  #    Function           File                              Inbound Outbound Coupling  Risk
  ─────────────────────────────────────────────────────────────────────────────────────────
  1    Build              internal/graph/builder.go:27            6       13       78  MEDIUM
  2    Parse              internal/parser/golang.go:19           24        3       72  HIGH
  3    Analyze            internal/analyzer/blast_radius.go:85    5        7       35  MEDIUM
```

### `tracescope why` — Trace call paths

```bash
tracescope why runAnalyze Score          # how does runAnalyze reach Score?
tracescope why graph.Build analyzer.Score # qualified names
tracescope why Score runAnalyze --reverse # reverse direction
```

```
TraceScope — Call Path (2 hops)

  runAnalyze                     internal/cmd/analyze.go:43 [cmd]
    │
    │ CALLS
    ▼
  Analyze                        internal/analyzer/blast_radius.go:85 [analyzer]
    │
    │ CALLS
    ▼
  Score                          internal/analyzer/risk_scorer.go:40 [analyzer]
```

### `tracescope report` — HTML visualization

```bash
tracescope report --open                     # graph-only, open in browser
tracescope report --diff changes.patch       # with blast radius overlay
git diff | tracescope report -o report.html  # pipe diff
```

Generates a self-contained HTML file with an interactive D3.js force-directed graph. Supports zoom, pan, drag, search, filters, node detail panel, and blast radius overlay.

### `tracescope validate-scip` - Compare SCIP and parser graphs

```bash
tracescope validate-scip .
```

Builds a SCIP graph and a parser fallback graph for the same repo, then reports shared/missing/extra node and edge signatures. Use this to sanity-check SCIP ingestion quality before relying on SCIP blast-radius results.

## CI Integration

### Exit codes

| Code | Meaning |
|------|---------|
| 0    | No risk or only LOW risk |
| 1    | HIGH risk functions affected |
| 2    | MEDIUM risk functions affected |

### GitHub Actions

```yaml
- name: Index codebase
  run: tracescope index .

- name: Analyze blast radius
  run: git diff origin/main...HEAD | tracescope analyze --format json --top 20

- name: Post PR comment with owners
  run: git diff origin/main...HEAD | tracescope analyze --github-comment --owners
```

## Configuration

Create a `.tracescope.yaml` in your project root:

```yaml
ignore:
  - vendor/**
  - dist/**
  - node_modules/**

max_depth: 5
format: terminal
top: 20
graph_path: .tracescope/graph.json

risk:
  high_callers: 10
  high_exported_callers: 5
  medium_callers: 3
```

CLI flags always override config file values.

## Supported Languages

| Language   | Functions | Classes/Structs | Imports | Calls | Inheritance |
|------------|-----------|-----------------|---------|-------|-------------|
| Go         | functions, methods | structs, interfaces | package imports | direct + qualified | embedded structs/interfaces |
| JavaScript | functions, arrow functions, class methods | classes | import/require | direct + member | class extends |
| TypeScript | functions, arrow functions, class methods | classes, interfaces, types | import/require | direct + member | extends + implements |
| Python     | functions, methods (with decorators) | classes | import, from-import | direct + chained | class inheritance (multiple) |

## Graph Model

**Node types:** `function`, `file`, `class`

**Edge types:**

| Edge | Meaning |
|------|---------|
| CALLS | Function A calls function B |
| CONTAINS | File contains function/class, or class contains method |
| IMPORTS | File imports another file/module |
| EXTENDS | Class/struct extends/embeds another |
| IMPLEMENTS | Class implements interface |

## Architecture

```
cmd/tracescope/          CLI entry point
internal/
  cmd/                   Cobra commands (index, analyze, hotspots, why, report)
  config/                YAML config loader with walk-up discovery
  parser/                Parser fallback (Go via go/ast, JS/TS/Python via tree-sitter)
  graph/                 Graph types, SCIP ingestion, fallback builder, BFS query, pathfinder, persistence
  diff/                  Unified diff parser (sourcegraph/go-diff)
  analyzer/              Blast radius, risk scoring, hotspot analysis, diff mapping
  output/                Terminal, JSON, GitHub Markdown, HTML report formatters
  ownership/             Git blame integration, CODEOWNERS parser
```

## Testing

```bash
go test ./... -race -v
```
