# TraceScope

A CLI tool that parses codebases into dependency graphs and analyzes PR blast radius.

TraceScope uses [tree-sitter](https://tree-sitter.github.io/) for AST parsing to build accurate cross-file dependency graphs, then uses reverse BFS traversal to compute the blast radius of code changes.

## Features

- **Multi-language parsing** — Go, JavaScript, TypeScript, Python via tree-sitter AST
- **Dependency graph** — Functions, classes, files with CALLS/CONTAINS/IMPORTS edges
- **Blast radius analysis** — Finds all functions affected by a code change
- **Risk scoring** — HIGH/MEDIUM/LOW based on caller count and export visibility
- **Diff integration** — Accepts unified diffs via file or stdin (`git diff | tracescope analyze`)

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
go build -o bin/tracescope ./cmd/tracescope
```

Or install directly:

```bash
go install github.com/anurag/tracescope/cmd/tracescope@latest
```

## Usage

### Index a codebase

```bash
tracescope index .
tracescope index /path/to/project
tracescope index . -v  # verbose output
```

This walks the directory, parses all supported files, builds the dependency graph, and saves it to `.tracescope/graph.json`.

### Analyze blast radius

```bash
# From a diff file
tracescope analyze --diff changes.diff

# From git diff via stdin
git diff | tracescope analyze
git diff HEAD~1 | tracescope analyze
```

Example output:

```
TraceScope — Blast Radius Analysis

  Changed Files (1):
    internal/cmd/root.go

  Changed Functions (1):
    Execute (internal/cmd/root.go:19)

  Blast Radius (1 affected):

    LOW RISK (1):
      main (cmd/tracescope/main.go:11) [0 callers, depth 1 — internal function with few callers]

  Summary:
    Graph: 130 nodes, 234 edges
    Changed: 1 files, 1 functions
    Blast radius: 1 affected functions (depth 5)
    Risk: 0 high, 0 medium, 1 low
```

## Supported Languages

| Language   | Functions | Classes/Structs | Imports | Calls |
|------------|-----------|-----------------|---------|-------|
| Go         | functions, methods (with receivers) | structs, interfaces | package imports | direct + qualified |
| JavaScript | functions, arrow functions, class methods | classes | import/require | direct + member |
| TypeScript | functions, arrow functions, class methods | classes, interfaces, type aliases | import/require | direct + member |
| Python     | functions, methods (with class context) | classes | import, from-import | direct + attribute |

## Risk Levels

| Level  | Criteria |
|--------|----------|
| HIGH   | 10+ callers, or exported with 5+ callers |
| MEDIUM | 3-10 callers, or exported/public |
| LOW    | Internal function with few callers |

## Architecture

```
cmd/tracescope/          CLI entry point
internal/
  cmd/                   Cobra commands (index, analyze)
  parser/                Tree-sitter language parsers
    queries/             S-expression query files
  graph/                 Graph types, builder, query, persistence
  diff/                  Unified diff parser
  analyzer/              Blast radius analysis + risk scoring
  output/                Terminal output formatting
```

## Testing

```bash
go test ./... -race -v
```

## License

MIT
