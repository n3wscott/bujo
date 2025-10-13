# Repository Guidelines

## Project Structure & Module Organization
- `bujo.go` hosts the Cobra root; subcommands in `pkg/commands/` hand requests to runners under `pkg/runner/<feature>/`.
- The interactive TUI resides in `pkg/runner/tea/`. `ui.go` orchestrates modes and service integration; view-model logic is split into `internal/indexview/` (calendar + index) and `internal/detailview/` (stacked detail pane). The bottom bar component lives in `internal/bottombar/`. Regression suites (`ui_navigation_test.go`, `ui_refresh_test.go`) pin current behaviour.
- Persistence and configuration helpers stay in `pkg/store/`, while shared UI primitives and demos remain in `pkg/ui/`, `views/`, `next/`, and `list-simple/`. `pkg/store/watch.go` now wraps `fsnotify` so runners can subscribe to disk changes without reimplementing walkers.
- Domain types, glyphs, and printers are in `pkg/entry/`, `pkg/glyph/`, and `pkg/printers/`; feature helpers belong next to the runner they serve.

## Build, Test, and Development Commands
- `go build -o bujo .` — build the CLI locally.
- `go run . --help` / `go run . ui` — inspect command wiring or launch the TUI (Bubble Tea will attempt AltScreen).
- `go install tableflip.dev/bujo@latest` — install the latest release.
- `gofmt -s -w . && go vet ./...` — enforce formatting and vet checks.
- `GOCACHE=$(pwd)/.gocache go test ./pkg/runner/tea` — run the current TUI test suite without tripping legacy UI packages; add `-race -v` when debugging.
- `GOCACHE=$(pwd)/.gocache go test ./pkg/store` — verify the fsnotify-backed watcher and persistence helpers without touching global caches.

## Coding Style & Naming Conventions
- Always format with `gofmt`/`goimports`; group imports stdlib → third-party → internal.
- Exported identifiers use `PascalCase`, locals `camelCase`, package names stay short and lowercase.
- Cobra command descriptions are imperative. Prefer small helpers (e.g., `handleNormalKey`, `loadDetailSectionsWithFocus`) over monolithic switches, and comment intent only where logic is non-obvious.
- UI view-model code favours pure state transitions; rendering happens in dedicated components (`indexview`, `detailview`, `bottombar`).

## Testing Guidelines
- Co-locate tests (`*_test.go`) with the code they cover; use table-driven cases for runners, stores, and state helpers.
- Rely on in-memory fakes (see `fakePersistence` in tests) when touching persistence.
- Before refactors around calendar/index behaviour, extend `ui_navigation_test.go` or `ui_refresh_test.go` to lock expectations, then run `go test ./pkg/runner/tea`.

## Commit & Pull Request Guidelines
- Keep commit subjects imperative (“add today shortcut”), squash noisy checkpoints, and describe behavioural changes in the body.
- PRs should explain user-facing impact, link issues, and include screenshots/asciinema for TUI updates. Flag config/schema changes and refresh completions when keybindings or commands shift.

## Security & Configuration Tips
- Journals default to `~/.bujo.db`; configuration loads from `.bujo.yaml`. Respect `BUJO_` environment overrides (`BUJO_CONFIG_PATH`, `BUJO_PATH`) and never commit local artefacts.
- Validate user input before writing to the store to protect journal data.

## Architecture Overview
- CLI flow: Cobra command → runner (`pkg/runner/...`) → store/entries → printers/UI.
- The TUI is layered:
  - `ui.go` manages modes (normal, insert, command, etc.), service calls, and key dispatch.
  - `internal/indexview` renders the left-hand index/calendar and tracks fold state.
  - `internal/detailview` renders the right-hand stacked collection/day panes with natural scrolling (no sticky top).
  - `internal/bottombar` owns the contextual footer and command palette suggestions.
- The `:today` command jumps to the real `Month/Day` collection (no meta “Today” entry) and the app starts focused on today’s date by default.
- `store.Watch` streams fsnotify events; `app.Service.Watch` relays them so the TUI can invalidate caches and redraw in near real time (`watchEventMsg` → `handleWatchEvent`).
