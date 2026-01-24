# Bujo TUI QA Report

- Date: 2026-01-24 08:41:45
- QA source: /Users/n3wscott/src/n3wscott/bujo/QA.md
- BUJO_PATH: (not set)
- BUJO_CONFIG_PATH: (not set)

## Results Checklist

### A) Startup / Layout Smoke

- [ ] App launches without errors, shows nav (left) and detail (right), status bar at bottom.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Status line shows “Journal loaded”.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Resize terminal wider and narrower; layout should adjust (no overlap or truncated bottom bar).
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

### B) Command Bar + Overlays

- [ ] Press `:` to open command input; suggestions show and update.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Use `Tab` / `Shift+Tab` or `Down` / `Up` to move suggestion selection.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Press `Enter` to accept a suggestion, and `Esc` to exit command input.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] `:help` opens overlay; `Esc` closes and focus restores.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] `:debug` toggles the event viewer; the body should resize and the viewer appears/disappears.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] `:report 7d` opens report overlay; `Esc` closes it.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] `:today` and `:future` jump to the expected collection (verify status updates).
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

### C) Navigation List (Left Pane)

- [ ] Arrow keys / `j`/`k` move selection line-by-line.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] `Right` / `l` expands a folded node; `Left` / `h` collapses it.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Current month appears even if it has only placeholders; today is visible near the top of daily list.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Daily collections are sorted with today on top, then prior days (verify order).
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Selecting a month shows its days; selecting a day updates the detail pane.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Rapidly press `Down` 10+ times in nav: detail should update to the corresponding collection each time without jumps or skipped sections.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Move `Up` and `Down` across the boundary between months; detail should follow without resetting to an unexpected section.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] When hovering over a day in the nav (without Enter), detail should preview that day without changing selection in the nav.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

### D) Detail Pane (Right Pane)

- [ ] `Tab` moves focus to detail; `Shift+Tab` returns focus to nav.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Arrow keys / `j`/`k` move between bullets.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] `PageUp`/`PageDown` or `b`/`f` scrolls pages without losing selection.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] `Home`/`g` jumps to top; `End`/`G` jumps to bottom.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] If a day has no entries (e.g., `$QA_TOMORROW`), detail shows the placeholder message.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] **Known issue check:** pressing `Down` should not loop to the top or jump to a different day. If it does, record the collection name and steps.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] With detail focused, press `Down` repeatedly from the first bullet to the last; the cursor should advance sequentially with no skips.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] After reaching the last bullet, press `Down` once more; selection should stay at the last item (no jump to top).
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Press `Up` repeatedly from the first bullet; selection should stay at the first item (no wrap).
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Verify detail section order matches nav order for the current month (same day sequence as nav list).
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

### E) Add Task Overlay

- [ ] With nav focused on today, press `i` to open the Add Task overlay.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Cursor should align with the input field (no offset/ghost cursor). **Known issue check.**
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] `Tab` cycles fields; `Shift+Tab` goes backward.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Enter a task message, submit with `Enter`.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] On return, focus should land on the newly created entry (not a different day). **Known issue check.**
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Press `i` again, then `Esc` → confirmation prompt; `y` discards and exits.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

### F) Bullet Actions (Detail Pane)

- [ ] Select a bullet and press `x` to complete; bullet updates to completed glyph.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Select a bullet and press `Backspace` to strike; bullet updates to dropped glyph.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Press `*`, `!`, `?` to set signifiers; press `|` to clear.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Press `Enter`/`Space` on a bullet to open the bullet detail overlay; verify info renders; `Esc` closes.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

### G) Move / Future / New Collection

- [ ] Select a bullet and press `>` to open move overlay.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Use nav to pick a different collection and press `Enter`; overlay closes and bullet moves.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] In move overlay, select “+ New Collection...” and create one; confirm move.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Select a bullet and press `<` to move to Future; verify status update and bullet removed from current list.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

### H) Migration Overlay

- [ ] Run `:migrate 14d` (or `:migrate 7d`).
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Use `<` to focus Future nav, `>` to focus target nav; ensure focus indicator changes.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Select a candidate and move it to a target; entry disappears from migration list.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Select “+ New Collection...” to create a target; verify it is created.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] `Esc` exits overlay and restores prior focus.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

### I) Lock / Unlock

- [ ] Select a bullet and run `:lock`; verify status update.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Run `:unlock`; verify status update and bullet becomes movable again.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

### J) Quit

- [ ] Run `:quit` to exit cleanly.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

## Known Issues to Validate (Explicit Checks)

- [ ] Add Task overlay cursor alignment (should match input field).
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Down-arrow in detail should not loop to top or change day unexpectedly.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

- [ ] Detail list ordering should match nav order for daily collections.
  Result: [ ] Pass  [ ] Fail  [ ] N/A
  Notes:

## Notes / Observations

## Summary

- Passed:
- Failed:
- N/A:

## Follow-ups
