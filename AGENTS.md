# Repository Guidelines

## Project Structure & Module Organization
- `bujo.go`: CLI entrypoint; module `tableflip.dev/bujo`.
- `pkg/commands`: Cobra commands (e.g., `add.go`, `get.go`, `version.go`, `completion.go`).
- `pkg/runner`: Command business logic (per feature directory).
- `pkg/store`: Config and persistence. Default DB path is `~/.bujo.db`.
- `pkg/ui`, `views/`, `next/`, `list-simple/`: TUI components and examples/demos.
- `pkg/printers`, `pkg/entry`, `pkg/glyph`: output, data types, and symbols.

## Build, Test, and Development Commands
```bash
# Build local binary
go build -o bujo .

# Run locally
go run . --help

# Install latest (per README)
go install tableflip.dev/bujo@latest

# Enable bash completion for current session
. <(bujo completion)

# Format and vet
gofmt -s -w . && go vet ./...

# Run tests (none yet in repo, for future)
go test ./... -race -v
```

## Coding Style & Naming Conventions
- Use `gofmt` defaults; keep imports organized by `goimports` if available.
- Package names: short, lowercase; files mirror command/feature (e.g., `task.go`).
- Exported identifiers: `PascalCase`; unexported: `camelCase`.
- Cobra commands: one file per command; short descriptions in imperative mood.

## Testing Guidelines
- Place tests next to code as `*_test.go` using Go’s `testing` package.
- Prefer table-driven tests for runners and store; mock external state where possible.
- Example: `go test ./pkg/store -race -v` for focused runs.

## Commit & Pull Request Guidelines
- Commits: concise, imperative subject lines (e.g., "fix flags", "add info command").
- PRs: clear description, usage examples (commands), linked issues, and screenshots/asciinema for TUI changes.
- Update docs (`README.md`) and completion behavior when flags or commands change.

## Security & Configuration Tips
- Config: `.bujo.yaml` (default path discovered via home); DB file is `.bujo.db` and is git-ignored.
- Environment: `BUJO_` prefix is respected (e.g., `BUJO_PATH`), and `BUJO_CONFIG_PATH` adjusts the config search path.
- Do not commit local data; keep secrets out of config.

## Architecture Overview
- Flow: Cobra CLI → runners (`pkg/runner`) → store/entries → printers/UI.
