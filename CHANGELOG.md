# Changelog

Notable AxLang changes are recorded here. Versions follow semantic versioning.

## [0.2.0] - 2026-10-24

### Added

- Stage prerequisites with multiple dependencies, preserved in the runtime IR.
- Semantic detection of missing stage references and dependency cycles.
- `axls`, a stdio Language Server Protocol server with diagnostics, hover information, and formatting.
- A VS Code extension with AxLang syntax highlighting and `axls` integration.

### Changed

- AxLang formatting preserves stage dependencies and source comments.
- CI validates compiler and language-server behavior with race-enabled tests.

### Installation

```sh
go install github.com/vivek5200/axlang/cmd/axc@v0.2.0
go install github.com/vivek5200/axlang/cmd/axls@v0.2.0
```
