# TODO

## Refactor / Decompose
- [x] Split `pkg/tui/app/app.go` into smaller feature files (command handling, overlay lifecycle, migration/move flows, watch/cache, focus management). The `Update` method and multiple 80–120+ line handlers (`handleMoveSelection`, `showMigrateOverlay`, `handleMigrationMoveRequest`, etc.) are hard to reason about and should be broken into focused helpers.
- [x] Extract selection/highlight routing logic out of `pkg/tui/components/journal/model.go` and `pkg/tui/components/collectiondetail/model.go` into a small coordinator (or dedicated methods) to reduce cross-component knowledge and duplicated focus logic.
- [x] Split `pkg/tui/components/collectiondetail/model.go` into rendering/layout vs. event handling vs. data mutation; consider moving line-layout/scrolling into a smaller state struct (similar to `pkg/tui/components/detail/state.go`).
- [x] Split `pkg/tui/components/collectionnav/model.go` into list view, calendar view, and sorting/selection logic. The calendar-related methods (`calendarChildren`, `selectedCalendarDay`, `virtualDay`, etc.) can live in a small sub-module.
- [x] Break up `pkg/tui/components/detail/state.go` (820 lines) into render/layout, mutation, and navigation files; several long helpers (`formatEntryLines`, `ensureScrollVisible`, `renderSection`) are tightly coupled and difficult to test in isolation.
- [x] Split `pkg/tui/components/addtask/model.go` into input handling, collection filtering, and rendering; overlay logic is large and mixes view/behavior.
- [x] Split `pkg/tui/components/command/model.go` into suggestion overlay, input handling, and rendering; `refreshSuggestionOverlay` and `Update` are large and mix concerns.
- [x] Separate `pkg/tui/cache/cache.go` (state + events) from sync/diff helpers (`pkg/tui/cache/sync.go`) into clearer subpackages or files; the sync logic is complex and deserves its own unit-test surface.
- [x] Consider refactoring `pkg/store/diskv.go` into smaller helpers for meta, entries, and locking/error translation; file I/O paths are large and imperative.

## Logic Cleanup / Consistency
- [x] Consolidate collection ordering rules in one place (currently spread between `pkg/collection/viewmodel/viewmodel.go`, `pkg/tui/cache/sync.go`, and `pkg/tui/components/index/indexview.go`). Prefer a single canonical ordering function and reuse it across nav/detail/cache to avoid drift.
- [x] Normalize “now” handling: multiple subsystems call `time.Now()` independently. Inject a shared clock (e.g., via app/model options) so UI ordering, cache build, and nav calendar stay consistent.
- [x] Reduce repeated “overlay guard” patterns in `pkg/tui/app/app.go` by introducing a helper to handle “skip command/journal update” logic and a single overlay state machine entrypoint.
- [x] Improve selection/highlight interplay when nav is focused but detail is not (calendar highlight vs. detail view): centralize the rules for when detail should scroll/select vs. only highlight.
- [x] Standardize section/collection ID formatting in detail vs. nav (several helpers rely on formatted names); enforce a single canonical ID format to avoid mismatches.
- [x] Audit status message flows and make them deterministic (currently status is set from many branches in `app.go`).

## Tests To Add (Coverage Gaps)
- [x] Add tests for `pkg/tui/cache` (currently no tests):
  - [x] `buildBullets` ordering, parent-child linking, cycles/missing parents, dedupe behavior.
  - [x] `applySnapshotLocked` diff behavior (create/update/delete) and event emission.
  - [x] `CreateCollection`, `SyncCollection`, and error paths in `createBulletPersisted`.
- [x] Add tests for `pkg/tui/components/collectiondetail`:
  - [x] placeholder/empty-section rendering behavior, focus/scroll behavior when `cursor == -1`.
  - [x] selection retention after section reorder and after placeholder insertions.
  - [x] highlight vs. select event handling (day vs. non-day collections).
- [x] Add tests for `pkg/tui/components/collectionnav`:
  - [x] calendar day selection for missing entries (virtual days).
  - [x] highlight/select messages for day vs. non-day rows.
  - [x] `calendarChildren` ordering.
- [x] Add tests for `pkg/tui/components/index/indexview.go`:
  - [x] `BuildItems` ordering (future, daily w/ today-first, monthly, tracking, generic).
  - [x] `RenderCalendarRows` and `DefaultSelectedDay` edge cases.
- [x] Add tests for `pkg/tui/components/addtask` and `pkg/tui/components/command`:
  - [x] input handling transitions and overlay focus behavior.
- [x] Add tests for `pkg/store/diskv.go`:
  - [x] read/write round-trips, missing/corrupt file handling, concurrency edge cases.
- [x] Add tests for `pkg/commands` + `pkg/runner/*`:
  - [x] CLI option parsing and error paths (commands + runner integration).
- [x] Add tests for `pkg/collection/types.go` and `pkg/collection/meta.go` for type inference and path handling (minor, but currently uncovered).
  - [x] type inference and metadata (ParseType, GuessType, ValidateChildName, UnmarshalList).

## Docs / Comments / Style
- [x] Audit exported functions/types lacking doc comments in `pkg/tui/*` and `pkg/app/*`; add brief docs where public APIs are used across packages.
- [x] Remove or update stale inline comments in large UI models where behavior has shifted (e.g., overlay close behavior, focus handling).

## Architectural Follow-ups
- [x] Consider introducing small interfaces for service/cache interactions to enable unit-testing UI components without real disk/service access.
- [x] Move “testbed” utilities into a separate module or mark them clearly as non-production helpers to reduce noise in core packages.

## Bubble Tea Alignment Goals
- [x] Standardize key handling with `bubbles/key` maps in `collectionnav`, `collectiondetail`, `command`, and `addtask`; replace ad-hoc `msg.String()` switches with `key.Matches`.
- [x] Introduce a consistent child message routing pattern (e.g., `ChildMsg{From, Msg}`) or a small router helper in `pkg/tui/app` so parent/child message flow is explicit and testable.
- [x] Add a page/router layer in `pkg/tui/app` (single active view + overlay stack) to reduce the `Update` method size and make navigation explicit.
- [x] Refactor overlays into sub-models that implement `tea.Model`, with consistent focus/blur and message ownership.
- [x] Audit direct cross-component calls and replace with `pkg/tui/events` messages where practical to reinforce the event bridge.
- [x] Centralize Lip Gloss styling in `pkg/tui/theme` and replace inline styles in components where possible.
