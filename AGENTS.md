# Repository Guidelines

## Project Structure & Module Organization
- CLI entrypoint lives in `bujo.go`; Cobra commands under `pkg/commands/` delegate to feature runners in `pkg/runner/<feature>/`.
- The interactive TUI sits in `pkg/runner/tea/`: `ui.go` coordinates modes, while reusable index/calendar rendering is in `pkg/runner/tea/internal/indexview/`. Regression tests (`ui_refresh_test.go`, `ui_navigation_test.go`) guard these behaviours.
- Persistence and config utilities stay inside `pkg/store/`; shared UI primitives are under `pkg/ui/` with demos in `views/`, `next/`, and `list-simple/`.
- Domain types, glyphs, and printers remain in `pkg/entry/`, `pkg/glyph/`, and `pkg/printers/`. Keep feature-specific helpers colocated with their runner.

## Build, Test, and Development Commands
- `go build -o bujo .` — compile the CLI locally.
- `go run . --help` — inspect command wiring while iterating.
- `go install tableflip.dev/bujo@latest` — install the latest tagged release.
- `gofmt -s -w . && go vet ./...` — enforce formatting and vet checks.
- `GOCACHE=$(pwd)/.gocache go test ./pkg/runner/tea` — run the current TUI test suite without tripping older UI packages.

## Coding Style & Naming Conventions
- Always run `gofmt`/`goimports`; group imports by stdlib, third-party, internal.
- Exported identifiers stay `PascalCase`, locals `camelCase`, package names short and lowercase.
- Cobra command descriptions use imperative mood. Prefer small, testable helpers (e.g., mode handlers, service actions) over sprawling switches, and comment intent only when logic is non-obvious.

## Testing Guidelines
- Co-locate tests (`*_test.go`) with the code they validate; lean on table-driven cases for runners, stores, and view-model helpers.
- Use in-memory fakes (see `fakePersistence`) when touching the store; add calendar/index navigation cases before refactors.
- Run focused suites such as `go test ./pkg/runner/tea -race -v` ahead of TUI changes.

## Commit & Pull Request Guidelines
- Keep commit subjects imperative (“add today shortcut”), and squash noisy checkpoints.
- PRs should explain behaviour, link issues, flag config/schema changes, and include screenshots/asciinema for TUI updates.
- Update docs and completion scripts whenever commands, flags, or keybindings change.

## Security & Configuration Tips
- Journals default to `~/.bujo.db`; configs load from `.bujo.yaml`. Respect `BUJO_` overrides (`BUJO_CONFIG_PATH`, `BUJO_PATH`) and never commit local artefacts.
- Validate input before it reaches the store to protect user data.

## Architecture Overview
- Flow runs Cobra → runner (`pkg/runner/...`) → store/entries → printers/UI. The TUI is now layered, with orchestration in `ui.go` and view-model state in `internal/indexview/`; follow this split for future components.
