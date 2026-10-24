# Contributing to AxLang

Thanks for helping improve AxLang. Contributions should keep the compiler small, deterministic, and easy to build.

## Before opening a change

- Open an issue for a substantial language or compiler change so its compatibility impact can be discussed.
- Keep parser, semantic analysis, formatting, and output-target changes focused and covered by tests.
- Do not add third-party dependencies to this module; AxLang is intentionally standard-library-only.

## Local checks

From the repository root, run:

```sh
gofmt -w $(find . -name '*.go' -type f)
go vet ./...
go test -race ./...
go build ./cmd/axc
```

For parser changes, consider running the fuzz target as well:

```sh
go test -fuzz=Fuzz -fuzztime=30s ./parser
```

## Pull requests

Describe the user-visible change, any `.ax` syntax or output changes, and the checks you ran. Include a regression test for bug fixes. Keep generated binaries, local configuration, and credentials out of commits.
