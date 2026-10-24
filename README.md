# AxLang

[![CI](https://github.com/vivek5200/axlang/actions/workflows/ci.yml/badge.svg)](https://github.com/vivek5200/axlang/actions/workflows/ci.yml)

AxLang is a small, statically checked language for describing Astrix workspace verification, policy guardrails, bounded repair, and progressive deployment rules. Its compiler is implemented in Go and uses only the Go standard library.

## Features

- Hand-written lexer and recursive-descent/Pratt parser for `.ax` files.
- Source diagnostics with file, line, column, and highlighted spans.
- Semantic checks for stage references, heal-loop bounds, metric units, and canary steps.
- JSON execution-plan and OPA Rego v1 output targets.
- `axc fmt` formatter and `axc check` validator.

## Install

Requires Go 1.23 or newer.

```sh
go install github.com/vivek5200/axlang/cmd/axc@latest
```

Or build from a checkout:

```sh
go build -o axc ./cmd/axc
```

## Use

```sh
axc check examples/Astrixfile.ax
axc fmt examples/Astrixfile.ax
axc fmt --write examples/Astrixfile.ax
axc emit-ir examples/Astrixfile.ax
axc emit-rego examples/Astrixfile.ax
```

`fmt` writes formatted source to standard output unless `--write`/`-w` is specified. `check`, `emit-ir`, and `emit-rego` run semantic validation before producing results.

## Example

```hcl
stage "verify" {
  scope   = ast.blast_radius
  network = "none"
}

heal "triage" on failure(stage.verify) {
  max_attempts = 3
  allow when { actor.type == "autonomous_llm" and diff.modifies_tests("false") }
}
```

## Development

Run the compiler tests and static analysis from this directory:

```sh
go test -race ./...
go vet ./...
```

The parser fuzz target can be run with:

```sh
go test -fuzz=Fuzz -fuzztime=30s ./parser
```

See [Astrix](https://github.com/vivek5200/astrix) for the runtime that consumes the generated execution plan and policy modules.

## License

AxLang is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
