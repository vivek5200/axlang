# AxLang

[![CI](https://github.com/vivek5200/axlang/actions/workflows/ci.yml/badge.svg)](https://github.com/vivek5200/axlang/actions/workflows/ci.yml)

AxLang is a statically checked, standard-library-only Go compiler for Astrix Control Language (`.ax`) files. It provides source diagnostics, canonical formatting, a JSON execution-plan target, an OPA Rego v1 policy target, and a JSON-RPC Language Server Protocol server.

## Install

Requires Go 1.23 or newer.

```sh
go install github.com/vivek5200/axlang/cmd/axc@latest
go install github.com/vivek5200/axlang/cmd/axls@latest
```

Or build from this repository:

```sh
go build -o axc ./cmd/axc
go build -o axls ./cmd/axls
```

See the [changelog](CHANGELOG.md) for release history.

## Language tour

```hcl
workspace "payments" {
  languages = ["go", "typescript"]
}

stage "unit" {
  scope = ast.blast_radius
}

stage "integration" after(stage.unit) {
  scope = ast.blast_radius
}

guardrail "production-boundary" {
  deny when {
    actor.type == "autonomous_llm" and diff.touches("infra/**")
  }
}

heal "bounded-repair" on failure(stage.unit) {
  max_attempts = 3
  allow when { diff.modifies_tests("false") }
}

deploy "canary" after(stage.integration) {
  steps = [1%, 5%, 25%, 100%] every 2m
  rollback on { metrics.p95_latency > 250ms or metrics.http_5xx_rate > 0.5% }
}
```

The parser uses a linear hand-written lexer, recursive descent for declarations, and Pratt parsing for expressions. Static checks include stage reference integrity and cycle rejection (`AX002`), heal attempt bounds (`AX001`), metric unit consistency (`AX003`), and monotonic canary percentages ending at `100%` (`AX004`).

## EBNF

The grammar below describes the supported declaration and expression forms. Field values are expressions; semantic checks constrain fields such as `max_attempts` and `steps`.

```ebnf
file              = { declaration } ;
declaration       = workspace | guardrail | stage | heal | deploy ;
workspace         = "workspace" string "{" { field } "}" ;
guardrail         = "guardrail" string "{" { guardrail-rule } "}" ;
guardrail-rule    = ("deny" | "require_approval") "when" "{" expression "}" ;
stage             = "stage" string [ "after" stage-list ] "{" { field } "}" ;
stage-list        = "(" stage-ref { "," stage-ref } ")" ;
heal              = "heal" string "on" "failure" "(" stage-ref ")" "{" { field | allow-rule } "}" ;
allow-rule        = "allow" "when" "{" expression "}" ;
deploy            = "deploy" string "after" "(" stage-ref ")" "{" { field | rollback-rule } "}" ;
rollback-rule     = "rollback" "on" "{" expression "}" ;
field             = identifier "=" expression [ "every" duration ] ;
stage-ref         = "stage." identifier ;
expression        = or-expression ;
or-expression     = and-expression { "or" and-expression } ;
and-expression    = equality-expression { "and" equality-expression } ;
equality-expression = comparison-expression { ("==" | "!=") comparison-expression } ;
comparison-expression = call-expression { ("<" | ">" | "<=" | ">=") call-expression } ;
call-expression   = primary { "(" [ expression { "," expression } ] ")" } ;
primary           = identifier | string | number | duration | percent
                  | list | "(" expression ")" ;
list              = "[" [ expression { "," expression } ] "]" ;
duration          = number ("ms" | "s" | "m" | "h") ;
percent           = number "%" ;
```

Boolean precedence, from weakest to strongest: `or`, `and`, equality, comparison, function call.

## Compiler commands

```sh
axc check Astrixfile.ax
axc fmt Astrixfile.ax
axc fmt --write Astrixfile.ax
axc emit-ir Astrixfile.ax
axc emit-rego Astrixfile.ax
```

`check`, `emit-ir`, and `emit-rego` run parsing and semantic validation. The runtime IR keeps stage prerequisites in each stage's `after` list.

## Rego mapping

| AxLang construct | Rego v1 output |
| --- | --- |
| `deny when { A and B }` | A single `deny if` rule with both conditions in its body |
| `deny when { A or B }` | Two `deny if` rules, one for each alternative |
| `require_approval when { A }` | A `require_approval if` rule |
| `allow when { A }` in `heal` | An `allow if` rule |
| `diff.touches("path/**")` | Iterates `input.modified_files` and calls `glob.match` |
| `250ms`, `0.5%` | Numeric seconds and fractional ratio values |

## Language server and editor

Build `axls` and make it available on `PATH`, then install the extension from [`editors/vscode`](editors/vscode) (or package it with `vsce`). The extension also accepts a custom executable path under **AxLang: Server: Path**.

The server uses LSP JSON-RPC over stdio and supports full document synchronization, parser and semantic diagnostics, generated Rego hover cards on guardrail/heal headers, domain-type hover for metrics and unit literals, and document formatting.

## Development

```sh
go test -race ./...
go vet ./...
go test -run '^$' -fuzz=FuzzParserAndSema -fuzztime=30s ./parser
```

AxLang has no third-party Go dependencies. See [Astrix](https://github.com/vivek5200/astrix) for the runtime consuming its plans and policy modules.

See [ARCHITECTURE.md](ARCHITECTURE.md) for package boundaries and compiler/LSP data flow.

## License

Apache License 2.0. See [LICENSE](LICENSE).
