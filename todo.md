# TODO

## Refactor / Decompose
- [ ] Split `pkg/tui/app/app.go` into smaller feature files (command handling, overlay lifecycle, migration/move flows, watch/cache, focus management). The `Update` method and multiple 80–120+ line handlers (`handleMoveSelection`, `showMigrateOverlay`, `handleMigrationMoveRequest`, etc.) are hard to reason about and should be broken into focused helpers.
- [ ] Extract selection/highlight routing logic out of `pkg/tui/components/journal/model.go` and `pkg/tui/components/collectiondetail/model.go` into a small coordinator (or dedicated methods) to reduce cross-component knowledge and duplicated focus logic.
- [ ] Split `pkg/tui/components/collectiondetail/model.go` into rendering/layout vs. event handling vs. data mutation; consider moving line-layout/scrolling into a smaller state struct (similar to `pkg/tui/components/detail/state.go`).
- [ ] Split `pkg/tui/components/collectionnav/model.go` into list view, calendar view, and sorting/selection logic. The calendar-related methods (`calendarChildren`, `selectedCalendarDay`, `virtualDay`, etc.) can live in a small sub-module.
- [ ] Break up `pkg/tui/components/detail/state.go` (820 lines) into render/layout, mutation, and navigation files; several long helpers (`formatEntryLines`, `ensureScrollVisible`, `renderSection`) are tightly coupled and difficult to test in isolation.
- [ ] Split `pkg/tui/components/addtask/model.go` into input handling, collection filtering, and rendering; overlay logic is large and mixes view/behavior.
- [ ] Split `pkg/tui/components/command/model.go` into suggestion overlay, input handling, and rendering; `refreshSuggestionOverlay` and `Update` are large and mix concerns.
- [ ] Separate `pkg/tui/cache/cache.go` (state + events) from sync/diff helpers (`pkg/tui/cache/sync.go`) into clearer subpackages or files; the sync logic is complex and deserves its own unit-test surface.
- [ ] Consider refactoring `pkg/store/diskv.go` into smaller helpers for meta, entries, and locking/error translation; file I/O paths are large and imperative.

## Logic Cleanup / Consistency
- [ ] Consolidate collection ordering rules in one place (currently spread between `pkg/collection/viewmodel/viewmodel.go`, `pkg/tui/cache/sync.go`, and `pkg/tui/components/index/indexview.go`). Prefer a single canonical ordering function and reuse it across nav/detail/cache to avoid drift.
- [ ] Normalize “now” handling: multiple subsystems call `time.Now()` independently. Inject a shared clock (e.g., via app/model options) so UI ordering, cache build, and nav calendar stay consistent.
- [ ] Reduce repeated “overlay guard” patterns in `pkg/tui/app/app.go` by introducing a helper to handle “skip command/journal update” logic and a single overlay state machine entrypoint.
- [ ] Improve selection/highlight interplay when nav is focused but detail is not (calendar highlight vs. detail view): centralize the rules for when detail should scroll/select vs. only highlight.
- [ ] Standardize section/collection ID formatting in detail vs. nav (several helpers rely on formatted names); enforce a single canonical ID format to avoid mismatches.
- [ ] Audit status message flows and make them deterministic (currently status is set from many branches in `app.go`).

## Tests To Add (Coverage Gaps)
- [x] Add tests for `pkg/tui/cache` (currently no tests):
  - [x] `buildBullets` ordering, parent-child linking, cycles/missing parents, dedupe behavior.
  - [x] `applySnapshotLocked` diff behavior (create/update/delete) and event emission.
  - [x] `CreateCollection`, `SyncCollection`, and error paths in `createBulletPersisted`.
- [ ] Add tests for `pkg/tui/components/collectiondetail`:
  - [x] placeholder/empty-section rendering behavior, focus/scroll behavior when `cursor == -1`.
  - [ ] selection retention after section reorder and after placeholder insertions.
  - [ ] highlight vs. select event handling (day vs. non-day collections).
- [ ] Add tests for `pkg/tui/components/collectionnav`:
  - [x] calendar day selection for missing entries (virtual days).
  - [ ] highlight/select messages for day vs. non-day rows.
  - [ ] `calendarChildren` ordering and `DefaultSelectedDay` logic.
- [ ] Add tests for `pkg/tui/components/index/indexview.go`:
  - [ ] `BuildItems` ordering (future, daily w/ today-first, monthly, tracking, generic).
  - [ ] `RenderCalendarRows` and `DefaultSelectedDay` edge cases (month boundaries, no days).
- [ ] Add tests for `pkg/tui/components/addtask` and `pkg/tui/components/command`:
  - [ ] input handling transitions and overlay focus behavior.
- [ ] Add tests for `pkg/store/diskv.go`:
  - [ ] read/write round-trips, missing/corrupt file handling, concurrency edge cases.
- [ ] Add tests for `pkg/commands` + `pkg/runner/*`:
  - [ ] CLI option parsing and error paths (commands + runner integration).
- [ ] Add tests for `pkg/collection/types.go` and `pkg/collection/meta.go` for type inference and path handling (minor, but currently uncovered).

## Docs / Comments / Style
- [ ] Audit exported functions/types lacking doc comments in `pkg/tui/*` and `pkg/app/*`; add brief docs where public APIs are used across packages.
- [ ] Remove or update stale inline comments in large UI models where behavior has shifted (e.g., overlay close behavior, focus handling).

## Architectural Follow-ups
- [ ] Consider introducing small interfaces for service/cache interactions to enable unit-testing UI components without real disk/service access.
- [ ] Move “testbed” utilities into a separate module or mark them clearly as non-production helpers to reduce noise in core packages.

## Bubble Tea Alignment Goals
- [ ] Standardize key handling with `bubbles/key` maps in `collectionnav`, `collectiondetail`, `command`, and `addtask`; replace ad-hoc `msg.String()` switches with `key.Matches`.
- [ ] Introduce a consistent child message routing pattern (e.g., `ChildMsg{From, Msg}`) or a small router helper in `pkg/tui/app` so parent/child message flow is explicit and testable.
- [ ] Add a page/router layer in `pkg/tui/app` (single active view + overlay stack) to reduce the `Update` method size and make navigation explicit.
- [ ] Refactor overlays into sub-models that implement `tea.Model`, with consistent focus/blur and message ownership.
- [ ] Audit direct cross-component calls and replace with `pkg/tui/events` messages where practical to reinforce the event bridge.
- [ ] Centralize Lip Gloss styling in `pkg/tui/theme` and replace inline styles in components where possible.
