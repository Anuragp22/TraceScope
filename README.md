# TraceScope

A CLI tool that parses codebases into dependency graphs and analyzes PR blast radius — helping teams understand the ripple effect of code changes before they ship.

## Features

- **Multi-language parsing** — Go (stdlib `go/ast`), JavaScript, TypeScript, Python (tree-sitter)
- **Dependency graph** — Functions, classes, files with CALLS/CONTAINS/IMPORTS edges
- **Blast radius analysis** — Finds all functions affected by a code change
- **Risk scoring** — HIGH/MEDIUM/LOW based on caller count, depth, and export visibility
- **Hotspot detection** — Find the most coupled functions in your codebase
- **Call path tracing** — Show why function A depends on function B
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

## Usage

### Index a codebase

```bash
tracescope index .
tracescope index /path/to/project
tracescope index . -v  # verbose output
```

Walks the directory, parses all supported files, builds the dependency graph, and saves it to `.tracescope/graph.json`.

### Analyze blast radius

```bash
# From git diff via stdin
git diff | tracescope analyze
git diff HEAD~3 | tracescope analyze
git diff main..feature | tracescope analyze

# From a diff file
tracescope analyze --diff changes.patch

# Limit results
tracescope analyze --top 10 --depth 3

# Exclude files
tracescope analyze --ignore "vendor/**,dist/**"

# JSON output for CI
git diff | tracescope analyze --format json
```

Example output:

```
TraceScope — Blast Radius Analysis

  Changed Files (1):
    internal/graph/builder.go

  Changed Functions (1):
    Build (internal/graph/builder.go:26)

  Blast Radius (3 affected):

    MEDIUM RISK (2):
      TestBuilder_BasicGraph (internal/graph/builder_test.go:9) [0 callers, depth 1]
      TestBuilder_ClassNodes (internal/graph/builder_test.go:59) [0 callers, depth 1]

    LOW RISK (1):
      runIndex (internal/cmd/index.go:27) [0 callers, depth 1]

  Summary:
    Graph: 199 nodes, 413 edges
    Changed: 1 files, 1 functions
    Blast radius: 3 affected functions (depth 5)
    Risk: 0 high, 2 medium, 1 low
```

### Find hotspots

```bash
tracescope hotspots              # top 10 most coupled functions
tracescope hotspots --top 20     # show more
tracescope hotspots --lang go    # filter by language
tracescope hotspots --format json
```

Ranks functions by **coupling score** (inbound callers × outbound calls). Functions that are both heavily depended-upon and themselves heavily dependent are the riskiest to change.

```
TraceScope — Hotspot Analysis

  #    Function                       File                                      Inbound Outbound Coupling  Risk
  ─────────────────────────────────────────────────────────────────────────────────────────────────────────────
  1    walk                           internal/parser/typescript.go:59                3       13       39  LOW
  2    Analyze                        internal/analyzer/blast_radius.go:85            5        7       35  MEDIUM
  3    MapDiffToFunctions             internal/analyzer/diff_mapper.go:19             6        5       30  MEDIUM
  4    Score                          internal/analyzer/risk_scorer.go:40             8        3       24  MEDIUM
  5    Build                          internal/graph/builder.go:26                    3        8       24  LOW
```

### Trace call paths

```bash
tracescope why runAnalyze Score          # how does runAnalyze reach Score?
tracescope why graph.Build analyzer.Score # qualified names
tracescope why Score runAnalyze --reverse # reverse: who calls Score?
tracescope why main Score --format json
```

Shows the shortest call path between two functions:

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

- name: Post PR comment
  run: git diff origin/main...HEAD | tracescope analyze --github-comment
```

### GitHub PR Comments

Use `--github-comment` to automatically post (or update) a blast radius summary on the current PR. Requires the [`gh` CLI](https://cli.github.com/) to be installed and authenticated.

## Configuration

Create a `.tracescope.yaml` in your project root:

```yaml
# File patterns to always exclude
ignore:
  - vendor/**
  - dist/**
  - node_modules/**

# Analysis defaults
max_depth: 5
format: terminal
top: 20

# Custom graph output path
graph_path: .tracescope/graph.json

# Risk scoring thresholds
risk:
  high_callers: 10          # callers for HIGH risk
  high_exported_callers: 5  # callers for HIGH when exported
  medium_callers: 3         # callers for MEDIUM risk
```

CLI flags always override config file values.

## Supported Languages

| Language   | Functions | Classes/Structs | Imports | Calls |
|------------|-----------|-----------------|---------|-------|
| Go         | functions, methods (with receivers) | structs, interfaces | package imports | direct + qualified |
| JavaScript | functions, arrow functions, class methods | classes | import/require | direct + member |
| TypeScript | functions, arrow functions, class methods | classes, interfaces, type aliases | import/require | direct + member |
| Python     | functions, methods (with decorators) | classes | import, from-import (relative) | direct + chained attribute |

## Risk Levels

| Level  | Criteria |
|--------|----------|
| HIGH   | 10+ production callers, or exported with 5+ production callers |
| MEDIUM | 3+ production callers, or exported/public, or direct dependency |
| LOW    | Internal function with few callers |

Risk scoring excludes test callers — only production code counts toward thresholds.

## Architecture

```
cmd/tracescope/          CLI entry point
internal/
  cmd/                   Cobra commands (index, analyze, hotspots, why)
  config/                YAML config loader with walk-up discovery
  parser/                Language parsers (Go via go/ast, JS/TS/Python via tree-sitter)
  graph/                 Graph types, builder, BFS query, pathfinder, persistence
  diff/                  Unified diff parser
  analyzer/              Blast radius, risk scoring, hotspot analysis, diff mapping
  output/                Terminal, JSON, GitHub Markdown formatters
```

## Testing

```bash
go test ./... -race -v    # 62 tests with race detector
```
