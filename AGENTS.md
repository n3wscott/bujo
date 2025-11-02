# Repository Guidelines

## Project Structure & Module Organization
- `bujo.go` hosts the Cobra root; subcommands in `pkg/commands/` hand requests to runners under `pkg/runner/<feature>/`. Collection metadata helpers (type enums, validation, JSON marshalling) live in `pkg/collection/`, while CLI runners for that metadata sit in `pkg/runner/collections/`.
- The interactive TUI resides in `pkg/runner/tea/`. `ui.go` orchestrates modes and service integration; view-model logic is split into `internal/indexview/` (calendar + index) and `internal/detailview/` (stacked detail pane). The bottom bar component lives in `internal/bottombar/`. Regression suites (`ui_navigation_test.go`, `ui_refresh_test.go`) pin current behaviour.
- Persistence and configuration helpers stay in `pkg/store/`; calendar rendering lives alongside the TUI in `pkg/runner/tea/internal/calendar/`. `pkg/store/watch.go` wraps `fsnotify` so runners can subscribe to disk changes without reimplementing walkers.
- Domain types, glyphs, and printers are in `pkg/entry/`, `pkg/glyph/`, and `pkg/printers/`; feature helpers belong next to the runner they serve.
- Reporting logic lives in `pkg/app/report.go`; shared duration parsing sits in `pkg/timeutil/`. CLI wiring is in `pkg/commands/report.go`, while the TUI report overlay reuses `detailview` sections.

## Build, Test, and Development Commands
- `go build -o bujo .` — build the CLI locally.
- `go run . --help` / `go run . ui` — inspect command wiring or launch the TUI (Bubble Tea will attempt AltScreen).
- `go install tableflip.dev/bujo@latest` — install the latest release.
- `go run . report --last 3d` — list recently completed entries (window defaults to `1w`).
- `go run . collections type "Future" monthly` — set or create a collection with the requested type (monthly/daily/generic/tracking).
- `gofmt -s -w . && go vet ./...` — enforce formatting and vet checks.
- `GOCACHE=$(pwd)/.gocache go test ./pkg/runner/tea` — run the current TUI test suite; add `-race -v` when debugging.
- `GOCACHE=$(pwd)/.gocache go test ./pkg/store` — verify the fsnotify-backed watcher and persistence helpers without touching global caches.
- `GOCACHE=$(pwd)/.gocache go test ./pkg/timeutil` — validate duration parsing helpers before shipping new window keywords.

## Command Safety
- Never run `git checkout` (or `git checkout -- <file>`) to revert work unless the user explicitly requests it. Use non-destructive alternatives (or ask first) so in-progress context isn’t lost.

## Coding Style & Naming Conventions
- Always format with `gofmt`/`goimports`; group imports stdlib → third-party → internal.
- Exported identifiers use `PascalCase`, locals `camelCase`, package names stay short and lowercase.
- Cobra command descriptions are imperative. Prefer small helpers (e.g., `handleNormalKey`, `loadDetailSectionsWithFocus`) over monolithic switches, and comment intent only where logic is non-obvious.
- UI view-model code favours pure state transitions; rendering happens in dedicated components (`indexview`, `detailview`, `bottombar`).

## Testing Guidelines
- Co-locate tests (`*_test.go`) with the code they cover; use table-driven cases for runners, stores, and state helpers.
- Rely on in-memory fakes (see `fakePersistence` in tests) when touching persistence.
- Before refactors around calendar/index behaviour, extend `ui_navigation_test.go` or `ui_refresh_test.go` to lock expectations, then run `go test ./pkg/runner/tea`.
- Report generation and collection-type inference tests live in `pkg/app/app_test.go` alongside the in-memory persistence fake; prefer those helpers when tweaking heuristics or adding report coverage.

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
- Collection types drive rendering: `monthly` parents (e.g., `Future`) expand into month folders, `daily` months render the calendar grid, `tracking` collections group under a synthetic footer panel. Both the CLI (`bujo collections type <name> <type>`) and TUI commands (`:type [collection] <type>`, `:new-collection`) call into `Service.SetCollectionType`, which enforces naming rules before persisting. `EnsureCollections` and `EnsureCollectionOfType` infer types for legacy data, ensuring calendar folders upgrade without manual edits.
- `Service.Report` groups completed entries by collection within a window; it powers both `bujo report --last <duration>` and the TUI's scrollable `:report` overlay. (TODO: expose alternate report output formats such as JSON/Markdown.)
- The TUI code now lives under `pkg/tui/`:
  - `pkg/tui/app` hosts the Bubble Tea root model/tests.
  - `pkg/tui/components/…` contains reusable panes (`index`, `detail`, `bottombar`, `panel`, `calendar`), each implementing the shared `pkg/tui/ui.Component` interface.
  - `pkg/tui/views/…` contains workflow overlays (`wizard`, `report`, `migration`) that the root model composes like any other component.
  - `pkg/tui/theme` owns Lip Gloss styles; `pkg/tui/uiutil` centralizes formatting helpers.
  - `pkg/runner/tea` is now a thin shim that calls into `pkg/tui/app` to keep the CLI wiring stable.
- The testbed CLI mirrors the component structure: shared harness logic stays in `testbed/main.go`, while feature-specific commands (e.g., `calendar`) live in their own files (see `testbed/calendar_cmd.go`) so we can iterate on individual components without bloating the main entrypoint.
- The TUI shares styling via `pkg/runner/tea/internal/theme`: extend this `Theme` struct when adding components so Lip Gloss styles stay centralized. Overlays such as the command footer, detail panel, report view, and future views should consume these semantic styles instead of instantiating `lipgloss.NewStyle` inline.
- Leaf UI pieces should implement the lightweight `ui.Component` interface (`Init`, `Update`, `View`, `SetSize`). Views like the collection wizard (`internal/views/wizard`) and migration dashboard (`internal/views/migration`) now live beside the report overlay, encapsulating their state/rendering so `ui.go` only handles routing and mode transitions.
- Shared formatting helpers belong in `internal/uiutil` (collection labels, entry labels, day parsing, etc.) to keep rendering logic consistent between the root model and the new component packages.

## Bubble Tea at scale: structuring large TUIs
- **Repo layout:** keep TEA’s root model under `internal/app` (model/update/view), generic widgets in `internal/ui`, workflow-specific “views” in `internal/views`, and platform ports/adapters separated so models remain pure. Add `theme.go` to host Lip Gloss styles and a shared `Theme` struct. (See Bubble Tea docs on Go Packages.)
- **Components:** define a tiny `Component` interface (`Init`, `Update`, `View`, `SetSize`) so parent models can compose leaf widgets. Components should emit typed messages (e.g., `SelectedMsg`) and accept dependencies via constructor options—never global state. Provide `SetSize` so only the root handles `tea.WindowSizeMsg`.
- **Reusability:** wrap Bubbles primitives (list, table, textinput, viewport, help) with your theme and messages. Package reusable components under `pkg/` with `New(opts ...)`, typed messages, and versioned modules if you plan to share them across repos.
- **Styling:** centralize Lip Gloss style definitions in a `Theme` and avoid hardcoding colors. Provide layout helpers (`Gap`, `Pad`, `JoinH/V`) and keep width/height math in the root. Renderers stay stateless; pair Lip Gloss with reflow/viewport for ANSI-aware wrapping.
- **Navigation:** treat the app as a tree of models. The root routes `Msg`s to children, aggregates `Cmd`s, and manages view stacks/routes. Use typed wrapper messages (`ChildMsg{From, Msg}`) to bubble events up. Focus management is just “send key to focused child first, others can ignore”. Router patterns: single active view, stack of views, or dashboards (broadcast messages, let inactive children drop them).
- **Message routing & subscriptions:** parent handles global input (`WindowSizeMsg`, quit), broadcasts domain messages, and listens for child outputs. Commands perform IO; state updates stay fast/pure. Combine child commands with `tea.Batch`.
- **Testing/logging:** drive interactive flows via teatest, log message streams when debugging, benchmark `View()` for large lists. Keep models pure to simplify unit tests of view-model logic (`Update` → new state + command).
- **Pitfalls to avoid:** monolithic “god” model (split into nested components), scattered layout math (centralize), blocking IO in `Update` (use `Cmd`s), inconsistent UX (share keymaps/help/theme). Bubble Tea community tips (leg100) echo these best practices.

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
- Avoid running interactive Bubble Tea binaries (e.g., `go run .`, `go run . ui`) unless explicitly requested by the user; doing so can lock the terminal during automation.
