# Repository Guidelines

## Project Structure & Module Organization
- `bujo.go` hosts the Cobra entrypoint for module `tableflip.dev/bujo`.
- Commands live in `pkg/commands/` (one file per verb), with business logic in `pkg/runner/<feature>/`.
- Persistence and config utilities are under `pkg/store/`; UI helpers live in `pkg/ui/` and demo views in `views/`, `next/`, and `list-simple/`.
- Data models, glyphs, and printers reside in `pkg/entry/`, `pkg/glyph/`, and `pkg/printers/`.
- Keep new features aligned with this layout; colocate tests as `*_test.go` next to the code they exercise.

## Build, Test, and Development Commands
- `go build -o bujo .` — compile the CLI locally.
- `go run . --help` — preview commands and flags during development.
- `go install tableflip.dev/bujo@latest` — install the latest tagged release to `$GOBIN`.
- `gofmt -s -w . && go vet ./...` — format sources and vet for common issues.
- `go test ./... -race -v` — execute the future test suite with the race detector enabled.

## Coding Style & Naming Conventions
- Run `gofmt` (or `goimports`) before sending changes; keep imports grouped by standard, third-party, project.
- Use `PascalCase` for exported symbols, `camelCase` for locals, and short lowercase package names.
- Cobra command files stay concise; descriptions use present-tense imperatives (e.g., "Add a task").
- Prefer sparse comments that explain intent around non-obvious logic paths.

## Testing Guidelines
- Use Go’s `testing` package with table-driven cases for runners and store interactions.
- Name tests `Test<Type>` and helpers `test<Type>`; keep fixtures in-memory unless persistence coverage is required.
- Run focused suites (`go test ./pkg/store -race -v`) before pushing changes that touch persistence.

## Commit & Pull Request Guidelines
- Write commits with imperative subjects (e.g., "add info command"), and keep related changes squashed.
- Pull requests should explain feature intent, link issues, note config or schema changes, and attach screenshots/asciinema for TUI updates.
- Update docs and completion scripts whenever flags, commands, or defaults change.

## Security & Configuration Tips
- Default database lives at `~/.bujo.db`; configuration loads from `.bujo.yaml` in the discovered home path.
- Support overrides through `BUJO_` prefixed env vars (e.g., `BUJO_CONFIG_PATH`, `BUJO_PATH`); never commit local data.
- Validate inputs that touch the store to avoid corrupting user journals.

## Architecture Overview
- Request flow: Cobra CLI parses input → runners in `pkg/runner/` orchestrate work → stores and entries handle persistence → printers/UI shape output.
- Align new features with this pipeline to keep separation of concerns clear.
