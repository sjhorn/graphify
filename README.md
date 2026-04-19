# graphify

[![CI](https://github.com/sjhorn/graphify/actions/workflows/ci.yml/badge.svg)](https://github.com/sjhorn/graphify/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/sjhorn/graphify/branch/main/graph/badge.svg)](https://codecov.io/gh/sjhorn/graphify)
[![Go Report Card](https://goreportcard.com/badge/github.com/sjhorn/graphify)](https://goreportcard.com/report/github.com/sjhorn/graphify)
[![Go Reference](https://pkg.go.dev/badge/github.com/sjhorn/graphify.svg)](https://pkg.go.dev/github.com/sjhorn/graphify)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A Go implementation of [graphify](https://github.com/safishamsi/graphify) — turn source code into a knowledge graph, cluster it into communities, and generate a visual report.

## What it does

1. **Detect** source files in a project directory
2. **Extract** nodes (functions, classes, modules) and edges (calls, inherits, imports) using tree-sitter AST parsing
3. **Build** a knowledge graph from the extracted entities
4. **Cluster** the graph into communities using the Louvain algorithm
5. **Analyze** the graph for god nodes and surprising connections
6. **Export** an interactive HTML visualization, JSON graph, and Markdown report

## Supported languages (24)

C, C++, C#, Dart, Elixir, Elm, Go, Haskell, Java, JavaScript/TypeScript (JSX/TSX), Julia, Kotlin, Lua, Objective-C, PHP, PowerShell, Python, R, Ruby, Rust, Scala, Svelte, Swift, Vue, Zig

## Installation

```bash
go install github.com/sjhorn/graphify/cmd/graphify@latest
```

## Usage

```bash
# Analyze a project directory
graphify /path/to/project

# Specify output directory
graphify -out results /path/to/project

# Verbose output
graphify -verbose /path/to/project
```

## Output

All output is written to the output directory (`graphify-out/` by default):

- **graph.json** — full graph data with nodes, edges, and community assignments
- **graph.html** — interactive force-directed visualization
- **GRAPH_REPORT.md** — Markdown report with community summaries, god nodes, and surprising connections
## How extraction works

Each source file is parsed into a tree-sitter AST. Language-specific extractors walk the AST to identify:

- **Nodes**: files, classes, functions, methods, interfaces, enums, modules
- **Edges** with relation types:
  - `contains` — file contains a class, class contains a method
  - `method` — class has a method
  - `calls` — function calls another function
  - `inherits` — class extends or implements another
  - `imports` — file imports a module or symbol
  - `case_of` — enum variant belongs to an enum

## Using with Claude Code

Add this snippet to your project's `CLAUDE.md` to instruct Claude Code agents to leverage graphify during explore and research phases:

```markdown
## Codebase exploration with graphify

When exploring this codebase for the first time, or when researching how components connect:

1. Build the knowledge graph (once per session):
   ```bash
   graphify /path/to/this/project
   ```

2. Read the architecture overview:
   - `graphify-out/GRAPH_REPORT.md` — god nodes, communities, design patterns, dependency layers

3. Use subcommands for targeted exploration:
   ```bash
   # Natural language graph traversal (budget = max output tokens)
   graphify query "How does authentication work?" --budget 3000

   # Trace connections between two entities
   graphify path "AuthService" "UserRepository"

   # Deep dive on a specific node
   graphify explain "DatabaseClient"
   ```

Prefer graphify over grepping when the question is architectural ("how does X connect to Y?",
"what depends on Z?", "what are the core abstractions?").
```

## Attribution

This is a Go rewrite of [graphify](https://github.com/safishamsi/graphify) by Safi Shamsi.

Key dependencies:
- [tree-sitter/go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter) — AST parsing
- [gonum](https://gonum.org) — graph algorithms and community detection

## License

[MIT](LICENSE)
