# AxLang architecture

## Compiler pipeline

```text
source text
  → lexer (tokens + UTF-8 byte spans)
  → recursive-descent declaration parser
  → Pratt expression parser
  → semantic checks
  ├── formatter
  ├── JSON execution plan
  └── OPA Rego v1 module
```

`token` owns token kinds and source spans. `lexer` performs one forward scan without regular expressions or third-party packages. `parser` handles declaration boundaries and precedence-aware expressions. `sema` checks loop bounds, stage references and dependency cycles, domain-unit comparisons, and canary progression. Compiler diagnostics share source spans with the IDE server.

## Stage dependency model

Each stage stores zero or more named prerequisites. Semantic validation first checks references, then uses a depth-first three-color traversal to find dependency cycles and reports `AX002` at every stage in each detected cycle. The formatter preserves dependencies in source order; the runtime plan serializes them as `stages[].after`.

## Targets

- `targets/ir` converts the checked AST to the versioned JSON-compatible execution plan consumed by Astrix.
- `targets/rego` emits Rego v1 for guardrail deny/approval rules and heal allow rules. Boolean OR becomes multiple rules; conjunctions remain in one rule body.
- `formatter` prints stable canonical source and retains line comments.

## Language server

`lsp` implements Content-Length framed JSON-RPC over stdio without external Go dependencies. Documents are kept in memory and parsed on open/change. The server reports parser and semantic diagnostics, compiles policy declarations for Rego hover cards, infers metric and unit-literal hover types, and returns formatter text edits. `cmd/axls` provides the executable entry point.

## Dependency and test boundaries

The `axlang` Go module has no third-party dependencies. Parser, semantic, formatter, Rego, IR, and LSP packages have isolated tests; `FuzzParserAndSema` exercises arbitrary source input.
