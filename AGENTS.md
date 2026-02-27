# Repository Guidelines

## Project Structure & Module Organization
- `bujo.go` hosts the Cobra root; subcommands in `pkg/commands/` hand requests to runners under `pkg/runner/<feature>/`. Collection metadata helpers (type enums, validation, JSON marshalling) live in `pkg/collection/`, while CLI runners for that metadata sit in `pkg/runner/collections/`.
- The interactive TUI lives under `pkg/tui/` with the root Bubble Tea model in `pkg/tui/app/` and reusable panes in `pkg/tui/components/`. `pkg/runner/tea` is a thin shim that invokes the TUI app so CLI wiring stays stable.
- Persistence and configuration helpers stay in `pkg/store/`. `pkg/store/watch.go` wraps `fsnotify` so runners can subscribe to disk changes without reimplementing walkers.
- Domain types, glyphs, and printers are in `pkg/entry/`, `pkg/glyph/`, and `pkg/printers/`; feature helpers belong next to the runner they serve.
- Reporting logic lives in `pkg/app/report.go`; shared duration parsing sits in `pkg/timeutil/`. CLI wiring is in `pkg/commands/report.go`, while the TUI report overlay reuses `detailview` sections.

## Build, Test, and Development Commands
- `go build -o bujo .` — build the CLI locally.
- `go run . --help` / `go run . ui` — inspect command wiring or launch the TUI (Bubble Tea will attempt AltScreen).
- `go install tableflip.dev/bujo@latest` — install the latest release.
- `go run . api info --journal /tmp/bujo.db` — inspect the resolved machine runtime/config for an isolated journal.
- `go run . api entries list --journal /tmp/bujo.db --collection today` — list entries via the machine JSON contract.
- `go run . report --last 3d` — list recently completed entries (window defaults to `1w`).
- `go run . collections type "Future" monthly` — set or create a collection with the requested type (monthly/daily/generic/tracking).
- `gofmt -s -w . && go vet ./...` — enforce formatting and vet checks.
- `GOCACHE=$(pwd)/.gocache go test ./pkg/tui/...` — run the current TUI test suite; add `-race -v` when debugging.
- `GOCACHE=$(pwd)/.gocache go test ./pkg/store` — verify the fsnotify-backed watcher and persistence helpers without touching global caches.
- `GOCACHE=$(pwd)/.gocache go test ./pkg/timeutil` — validate duration parsing helpers before shipping new window keywords.

## Command Safety
- Never run `git checkout` (or `git checkout -- <file>`) to revert work unless the user explicitly requests it. Use non-destructive alternatives (or ask first) so in-progress context isn’t lost.

## Coding Style & Naming Conventions
- Always format with `gofmt`/`goimports`; group imports stdlib → third-party → internal.
- Exported identifiers use `PascalCase`, locals `camelCase`, package names stay short and lowercase.
- Cobra command descriptions are imperative. Prefer small helpers (e.g., `handleNormalKey`, `loadDetailSectionsWithFocus`) over monolithic switches, and comment intent only where logic is non-obvious.
- UI view-model code favours pure state transitions; rendering lives in dedicated components (`collectionnav`, `collectiondetail`, `bottombar`, etc.). Cross-pane communication should flow through typed events in `pkg/tui/events` (event bridge), not direct coupling.
- Keep `Update`/`View` work fast; move expensive operations into `tea.Cmd`s or background services so the event loop never stalls.

## Testing Guidelines
- Co-locate tests (`*_test.go`) with the code they cover; use table-driven cases for runners, stores, and state helpers.
- Rely on in-memory fakes (see `fakePersistence` in tests) when touching persistence.
- Before refactors around calendar/index behaviour, extend the component regression tests (for example `pkg/tui/components/.../_test.go` or `pkg/tui/app/command_layout_test.go`) and run `go test ./pkg/tui/...`.
- Report generation and collection-type inference tests live in `pkg/app/app_test.go` alongside the in-memory persistence fake; prefer those helpers when tweaking heuristics or adding report coverage.

## Commit & Pull Request Guidelines
- Keep commit subjects imperative (“add today shortcut”), squash noisy checkpoints, and describe behavioural changes in the body.
- PRs should explain user-facing impact, link issues, and include screenshots/asciinema for TUI updates. Flag config/schema changes and refresh completions when keybindings or commands shift.

## Security & Configuration Tips
- Journals default to `~/.bujo.db`; configuration loads from `.bujo.yaml`. Respect `BUJO_` environment overrides (`BUJO_CONFIG_PATH`, `BUJO_PATH`) and never commit local artefacts.
- Validate user input before writing to the store to protect journal data.

## Architecture Overview
- CLI flow: Cobra command → runner (`pkg/runner/...`) → store/entries → printers/UI.
- `bujo api` is the machine-oriented contract for agents/automation. It is JSON-first, returns structured success/error envelopes, supports explicit journal isolation via `--journal`, and includes selector-safe mutation semantics (`--id`/`--message`/`--collection`/`--type` with `--exact`/`--first`).
- Legacy human commands (`add`, `get`, `complete`, `strike`, etc.) remain human-oriented and print-friendly. Do not treat them as a stable machine contract; for automation, prefer `bujo api` rather than parsing text output.
- The TUI is layered:
  - `pkg/tui/app` hosts the Bubble Tea root model (`app.go`), command handling, and overlay orchestration.
  - `pkg/tui/components/collectionnav` renders the left-hand index/calendar and tracks fold state.
  - `pkg/tui/components/collectiondetail` renders the right-hand stacked collection/day panes with natural scrolling (no sticky top).
  - `pkg/tui/components/bottombar` owns the contextual footer and command palette suggestions.
- The `:today` command jumps to the real `Month/Day` collection (no meta “Today” entry), `:future` jumps to the Future log, and the app starts focused on today’s date by default.
- Use `:lock`/`:unlock` to toggle the immutable flag on the currently highlighted entry; locked tasks are skipped by migration, move, and future commands.
- `store.Watch` streams fsnotify events; `app.Service.Watch` relays them so the TUI can invalidate caches and redraw in near real time (`watchEventMsg` → `handleWatchEvent`).
- Collection types drive rendering: `monthly` parents (e.g., `Future`) expand into month folders, `daily` months render the calendar grid, `tracking` collections group under a synthetic footer panel. Both the CLI (`bujo collections type <name> <type>`) and TUI commands (`:type [collection] <type>`, `:new-collection`) call into `Service.SetCollectionType`, which enforces naming rules before persisting. `EnsureCollections` and `EnsureCollectionOfType` infer types for legacy data, ensuring calendar folders upgrade without manual edits.
- `Service.Report` groups completed entries by collection within a window; it powers both `bujo report --last <duration>` and the TUI's scrollable `:report` overlay. (TODO: expose alternate report output formats such as JSON/Markdown.)
- The TUI code now lives under `pkg/tui/`:
  - `pkg/tui/app` supplies the runnable Bubble Tea program and overlay implementations (`add_overlay.go`, `report_overlay.go`, `move_overlay.go`, etc.).
  - `pkg/tui/components/...` contains reusable panes (`index`, `detail`, `bottombar`, `calendar`, `overlaypane`, etc.) that implement the shared `pkg/tui/ui.Component` interface.
  - `pkg/tui/theme` owns Lip Gloss styles; `pkg/tui/uiutil` centralizes formatting helpers.
  - `pkg/runner/tea` is a thin shim that calls into `pkg/tui/app` to keep the CLI wiring stable.
- The testbed CLI mirrors the component structure: shared harness logic stays in `testbed/main.go`, while feature-specific commands (e.g., `calendar`) live in their own files (see `testbed/calendar_cmd.go`) so we can iterate on individual components without bloating the main entrypoint.
- The TUI shares styling via `pkg/tui/theme`: extend this `Theme` struct when adding components so Lip Gloss styles stay centralized. Overlays such as the command footer, detail panel, and report view should consume these semantic styles instead of instantiating `lipgloss.NewStyle` inline.
- Leaf UI pieces should implement the lightweight `ui.Component` interface (`Init`, `Update`, `View`, `SetSize`). Overlay panels such as add-task, bullet detail, move, and report live beside the root model inside `pkg/tui/app`, keeping routing/mode transitions in one place.
- Shared formatting helpers belong in `pkg/tui/uiutil` (collection labels, entry labels, day parsing, etc.) to keep rendering logic consistent between the root model and the component packages.
- Contract drift prevention: command docs in README are generated from the Cobra tree (`go run ./cmd/gendocs --write`) and enforced by contract tests (`go run ./cmd/gendocs --check`, `go test ./pkg/commands`).

## Bubble Tea at scale: structuring large TUIs
- **MVU-first routing:** treat `Update` as a message router; no blocking IO. Use `tea.Cmd` for side effects and keep state transitions fast/pure. Messages should represent “something happened.”
- **Composable models:** prefer sub-models that implement `tea.Model`. The parent delegates `Update`/`View` and aggregates `Cmd`s. Components emit typed messages with `ComponentID` (event bridge), not direct calls into siblings.
- **Event bridge discipline:** cross-pane coordination must flow through `pkg/tui/events` messages. Avoid tight coupling (don’t reach into other components’ internals to update state).
- **Keymaps:** define keymaps with `bubbles/key` and use `key.Matches` inside `Update`. Keep key handling consistent across components.
- **Reuse Bubbles patterns:** wrap Bubbles primitives (list, textinput, viewport, help) with the project theme and typed messages rather than custom widgets.
- **Router for scale:** for multiple workflows, add a small page/router layer (single active view, stack, or dashboard). Overlays should be modeled as sub-models with explicit focus/blur.
- **Styling/layout:** centralize Lip Gloss styles in `pkg/tui/theme` and keep layout math in the root model. Renderers stay stateless.
- **Async ordering:** `tea.Cmd`s run concurrently; never assume ordering. Tag/guard responses when state can race.
- **Testing/logging:** unit-test `Update` for view-model logic, and use testbed/teatest for integration flows. Add optional message logging for complex routing/debugging.

## Debugging & Recovery Tips
- When the event viewer isn’t enough, add an opt-in message logger (e.g., behind a `DEBUG` env var) that writes every `tea.Msg` to disk so you can tail interactions from another terminal.
- If a panic or forced quit leaves the terminal in raw mode, run `reset` to restore the cursor and echo before resuming work.

## Component/Testbed Notes
- `pkg/collection/viewmodel` converts flat `collection.Meta` data plus inferred children into hierarchical `ParsedCollection` structs, annotating month/day metadata, stable priority/sort keys, and daily day summaries so UI components don’t have to re-parse strings.
- `pkg/tui/components/collectionnav` now consumes those parsed collections, flattens them into multiple row kinds (monthly parents, daily months, day rows, tracking/generic lists), tracks fold state, and emits typed `SelectionMsg` events when rows are activated so parents can coordinate focus.
- Collection navigation also emits `HighlightMsg` (cursor moved) and `SelectMsg` (Enter/space activation) messages with a `ComponentID` so other panes can subscribe to `"MainNav"` vs other instances without hard coupling.
- The collection detail pane mirrors this: it emits `events.BulletHighlightMsg` when the cursor lands on a bullet and `events.BulletSelectMsg` on Enter/Space, tagging each message with `ComponentID` (e.g. `DetailPane`) plus section/bullet metadata for the event viewer.
- Detail panes now listen for `events.CollectionHighlightMsg` from their paired nav (`SetSourceNav`) and automatically scroll the appropriate section into view without clearing the other collections, matching the legacy behavior.
- In the journal composite, bullet highlight events bubble up to the nav so the left pane mirrors whichever collection the detail cursor is inside; nav focus state also drives the selection color so the purple highlight only appears when the nav actually has focus.
- Focus transitions are standardized: calling a component’s `Focus()`/`Blur()` returns a `tea.Cmd` that emits `events.FocusMsg`/`events.BlurMsg`, so the root model (and event viewer) always know which pane currently owns keyboard input.
- `pkg/tui/components/journal` composes the nav (≈24 columns) and detail panes side-by-side; `testbed journal` wires it up with sample data so we can iterate on cross-pane focus and layout quickly.
- `testbed` commands accept `--real` to hydrate nav/detail/journal with the current on-disk journal via `store.Load`/`app.Service`, replacing the baked-in fixtures when you need to repro bugs against real data.
- Shared selection/highlight events live in `pkg/tui/events`; they expose `CollectionRef` helpers plus `Describe()` implementations so the event viewer can show `MainNav highlight Inbox (monthly)` instead of raw struct dumps.
- The `testbed` binary keeps the harness in `testbed/main.go` and exposes per-component commands (`calendar`, `nav`, etc.) in dedicated files; each command builds or mocks the data the component expects (e.g., the nav command feeds parsed collection trees) so we can iterate on individual widgets without disturbing the rest of the CLI.
- Daily collections in `collectionnav` no longer list child days; instead they embed the shared `index.CalendarModel` output inline, so the nav view reuses the same interactive calendar behaviour as the main TUI while keeping folding logic centralized.
- The nav testbed renders a metadata bar (selected collection, type, row kind, parsed month/day, child counts) to make it easy to confirm parsed view-model data without instrumenting the main UI; use `go run ./testbed nav` to verify focus, folding, and calendar interaction.
- `pkg/tui/components/eventviewer` is a reusable log panel that captures Bubble Tea events (timestamp, source, formatted payload) with a bordered viewport; it keeps the newest entry pinned to the top so we can watch focus changes and key flow in real time.
- `testbed/main.go` centers the framed component near the top and pins the event viewer directly to the bottom edge of the terminal at full width, so the log feels like a console footer. When vertical space is tight we shave rows off the frame (never the log) but `contentSize()` still reports the inner frame dimensions for components.
- The testbed now targets Bubble Tea v2 cursor semantics: every model's `View` returns `(string, *tea.Cursor)` and parents are responsible for offsetting child cursor positions when adding borders, padding, or centering with `lipgloss.Place`. Use helpers such as `offsetCursor` to clone and shift coordinates rather than mutating child cursors in place.
- Text inputs (e.g. `pkg/tui/components/addtask`) use `textinput.Model.Cursor()` to expose real cursors. After styling, adjust `cursor.Position.X/Y` by the number of padding and border cells you add (for our add-task frame that's +3 horizontally and +2 vertically before the testbed frame applies its own offsets). When composing nested views always add offsets in the same function that injects whitespace so the cursor stays aligned.
- When interacting with the TUI for QA or reproduction, use the `$terminal-controller` skill to drive tmux sessions and capture output deterministically.
- When adding or modifying testbed commands that launch Bubble Tea programs (including short-lived overlays), explicitly call out in your notes if they were not run, since interactive sessions are skipped unless the user requests them.
