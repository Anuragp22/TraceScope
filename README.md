# TraceScope

A SCIP-backed PR blast-radius analyzer for code review.

TraceScope turns a diff into a ranked impact report:
`changed function -> dependency path -> affected code -> risk -> suggested reviewer`.

## Portfolio Pitch

**Problem:** code review usually shows what changed, but not what may break downstream.

**What this project does:** builds a repository dependency graph, maps a PR diff to changed
functions, traverses downstream dependencies, ranks impact risk, and generates reviewer-focused
GitHub PR comments.

**What is technically interesting:** SCIP ingestion, parser fallback, cross-file graph
construction, blast-radius traversal, call-path explanations, confidence-aware resolution
diagnostics, and CI-friendly output.

**Scope boundary:** TraceScope is a static-analysis prototype for PR impact analysis, not a
full compiler or a replacement for an LLM reviewer like CodeRabbit.

## Demo Output

```markdown
## TraceScope Blast Radius

**Risk:** HIGH
**Changed functions:** 2
**Affected functions:** 8
**Graph source:** scip

### Reviewer focus

1. `Build` - `internal/graph/builder.go:27`
   - Why path: `Build -> registerReferenceEdges -> addEdge`
   - Confidence: exact
   - Owners: `@graph-team`
   - Inspect: call resolution and import-edge behavior

2. `ComputeBlastRadius` - `internal/graph/pathfinder.go:18`
   - Why path: `Build -> ComputeBlastRadius`
   - Confidence: exact
   - Owners: `@platform-team`
   - Inspect: traversal depth, duplicate edge handling, and ranking changes
```

## Architecture

```mermaid
flowchart LR
  A["git diff / patch"] --> B["Diff Mapper"]
  C["SCIP indexers"] --> D["SCIP Graph Builder"]
  E["Tree-sitter + Go parser fallback"] --> F["Fallback Graph Builder"]
  D --> G["Dependency Graph"]
  F --> G
  G --> H["Blast Radius Analyzer"]
  H --> I["Risk Scoring + Why Paths"]
  I --> J["Terminal / JSON / GitHub PR Comment / HTML Report"]
```

## Features

- **SCIP-first indexing** with `scip-go` and `scip-typescript`, plus parser fallback
- **Multi-language graph model** for files, functions, classes, imports, calls, and inheritance
- **Blast-radius analysis** from changed functions to downstream impacted functions
- **Risk ranking** based on call depth, caller fan-in, exports, and propagation
- **Why-path explanations** for how a changed symbol reaches an affected symbol
- **Confidence diagnostics** for exact, heuristic, ambiguous, and unresolved edges
- **Ownership hints** from git blame and CODEOWNERS
- **CI/GitHub output** through terminal, JSON, exit codes, and PR comments
- **Optional HTML graph report** for visual exploration

## Installation

### Prerequisites

- Go 1.22+
- GCC for tree-sitter fallback parsing
- Optional SCIP indexers for higher-quality symbol resolution

```bash
go install github.com/sourcegraph/scip-go/cmd/scip-go@latest
npm install -g @sourcegraph/scip-typescript
npm install -g @sourcegraph/scip-python
```

**Windows note:** `scip-python` currently fails on native Windows in the published package,
so TraceScope skips it there. Use WSL/Linux CI if Python SCIP indexing matters.

### Build

```bash
git clone https://github.com/Anuragp22/TraceScope.git
cd TraceScope
go build -o tracescope ./cmd/tracescope
```

## Core Commands

### Build the graph

```bash
tracescope index .
```

Behavior:
- Uses `index.scip` if one already exists at the repo root
- Otherwise tries `scip-go`, `scip-typescript index`, and `scip-python index`
- Merges generated SCIP indexes from `.tracescope/scip/`
- Falls back to built-in parsers if SCIP is unavailable
- Writes `.tracescope/graph.json`

Example:

```text
TraceScope - indexing /repo

  Found 89 files across 2 languages
  Using SCIP index: /repo/.tracescope/scip/scip-go.scip
  Using SCIP index: /repo/.tracescope/scip/scip-typescript-web.scip
  Built graph: 721 nodes, 2075 edges

  Stats:
    source:      scip
    CONTAINS:    696
    CALLS:       1308
    IMPORTS:     67
    IMPLEMENTS:  4
```

### Analyze blast radius

```bash
git diff origin/main...HEAD | tracescope analyze --owners
git diff origin/main...HEAD | tracescope analyze --github-comment --owners
tracescope analyze --diff changes.patch --depth 3 --top 10
```

### Explain a dependency path

```bash
tracescope why runAnalyze Score
tracescope why graph.Build analyzer.Score
tracescope why Score runAnalyze --reverse
```

### Find hotspots

```bash
tracescope hotspots --top 20
tracescope hotspots --lang go
```

### Validate SCIP vs parser fallback

```bash
tracescope validate-scip .
```

This compares a SCIP graph against the parser fallback graph and reports shared, missing,
and extra node/edge signatures.

## GitHub Actions

```yaml
- name: Index codebase
  run: tracescope index .

- name: Analyze blast radius
  run: git diff origin/main...HEAD | tracescope analyze --format json --top 20

- name: Post PR comment
  run: git diff origin/main...HEAD | tracescope analyze --github-comment --owners
```

Exit codes:

| Code | Meaning |
|------|---------|
| 0 | No risk or only low-risk impact |
| 1 | High-risk impacted functions |
| 2 | Medium-risk impacted functions |

## Configuration

Create `.tracescope.yaml` in the repo root:

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

## Repository Layout

```text
cmd/tracescope/          CLI entry point
internal/cmd/            Cobra commands
internal/parser/         Parser fallback and file walking
internal/graph/          SCIP ingestion, parser graph builder, graph compare, BFS/path finding
internal/diff/           Unified diff parsing
internal/analyzer/       Diff-to-function mapping, blast-radius traversal, risk scoring, hotspots
internal/output/         Terminal, JSON, GitHub Markdown, HTML report
internal/ownership/      Git blame and CODEOWNERS
internal/server/         Local graph API and WebSocket server
web/                     Optional dashboard frontend
docs/                    Benchmarks and supporting notes
```

## Current Limitations

- Static analysis is still imperfect for highly dynamic JavaScript/Python patterns
- SCIP and parser fallback graphs do not match 1:1 because SCIP carries richer semantic edges
- `scip-python` is skipped on native Windows because of an upstream package issue
- The dashboard is demo-only; the main product surface is the PR blast-radius comment

## Testing

```bash
go test ./... -race -count=1 -timeout 120s
```

Benchmark notes are in [docs/benchmark-real-repo.md](docs/benchmark-real-repo.md).
