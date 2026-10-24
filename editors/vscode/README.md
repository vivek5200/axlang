# AxLang editor support

This VS Code-compatible extension provides `.ax` syntax highlighting and starts the AxLang language server (`axls`) for diagnostics, Rego hover previews, domain-type hover, and formatting.

## Build the language server

From a standalone AxLang checkout:

```sh
mkdir -p bin
go build -o bin/axls ./cmd/axls
```

From the dual-module workspace root, use `go build -o bin/axls ./axlang/cmd/axls` instead. Ensure the output directory is on `PATH`, or configure **AxLang: Server: Path** with the absolute path to the executable. Open an `.ax` file or `Astrixfile.ax` to activate the language client.

The extension declares `vscode-languageclient` as its editor-side dependency. The Go language server itself has no third-party Go dependencies.
