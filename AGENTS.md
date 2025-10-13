# Repository Guidelines

## Project Structure & Module Organization
- `bujo.go` hosts the Cobra root, with subcommands defined in `pkg/commands/`. Each command delegates to a runner under `pkg/runner/<feature>/`.
- The interactive TUI lives in `pkg/runner/tea`. `ui.go` coordinates modes and service calls, while supporting packages handle presentation: `internal/indexview/` (calendar + left index), `internal/detailview/` (stacked right-hand panes), `internal/bottombar/` (contextual footer/command palette), and `internal/calendar/` (calendar layout utilities). Regression suites `ui_navigation_test.go` and `ui_refresh_test.go` pin current behaviour.
- Persistence and configuration helpers reside in `pkg/store/`. Domain types, glyphs, and printers are in `pkg/entry/`, `pkg/glyph/`, and `pkg/printers/`.
- The MCP server lives in `pkg/runner/mcp`, exposing CLI capabilities to LLM clients via `mark3labs/mcp-go`. It registers tools/resources in `tools.go` and `resources.go`, with shared logic in `service.go`.

## Build, Test, and Development Commands
- `go build ./...` compiles the CLI and backing libraries.
- `go run . ui` launches the TUI; `go run . mcp` starts the MCP stdio server.
- `GOCACHE=$(pwd)/.gocache go test ./pkg/runner/tea` runs the TUI regression suite (add `-race -v` when debugging).
- `go test ./pkg/runner/mcp` exercises the MCP service helpers; new MCP work should extend these tests.
- `GOCACHE=$(pwd)/.gocache GOLANGCI_LINT_CACHE=$(pwd)/.golangci-cache golangci-lint run` enforces revive, staticcheck, govet, ineffassign, and errcheck.

## Coding Style & Naming Conventions
- Always format with `gofmt`/`goimports`; group imports stdlib → external → internal.
- Exported identifiers use doc comments (revive checks this). Keep files ASCII unless the existing file intentionally uses Unicode glyphs (the UI does).
- Prefer small, testable helpers (e.g., `handleNormalKey`, `moveCalendarCursor`, MCP `ParseBullet`) over monolithic switches; add short intent comments only where logic is non-obvious.
- When working in the MCP package, keep handlers thin—push persistence and validation into `service.go`.

## Testing Guidelines
- Co-locate `*_test.go` files with their packages. Use table-driven tests and in-memory fakes (see `fakePersistence` and `memoryStore`) for store interactions.
- Before adjusting calendar/index behaviour, extend `ui_navigation_test.go` or `ui_refresh_test.go` to lock expectations, then run `go test ./pkg/runner/tea`.
- Add MCP unit tests when introducing new tools/resources to prevent regressions in argument binding and DTO output.

## Commit & Pull Request Guidelines
- Keep commit subjects imperative (“add today shortcut”). Squash noisy checkpoints and describe behavioural changes in the body.
- PRs should explain user-facing impacts, list test commands, and include screenshots/asciinema for TUI updates. Flag config/schema changes and refresh completions when commands or keybindings shift.

## MCP Server Notes
- `bujo mcp` boots an MCP stdio server using `mark3labs/mcp-go`. Tools cover entry creation, completion, striking, moving, and updates; resources expose collections (`bujo://collections`, `bujo://collections/{name}`) and individual entries (`bujo://entries/{id}`).
- Handlers should remain stateless—use `Service` methods for persistence, keep argument validation close to the edge, and ensure structured results mirror CLI behaviour.
- Update README and this guide whenever new MCP capabilities land so LLM clients have accurate instructions.
