# Bujo TUI Manual QA Checklist

This checklist aims for ~90% coverage of common TUI interactions. It uses a fresh, deterministic QA dataset each run, seeded via CLI. Keep notes of any failures, visual glitches, or unexpected behavior (especially around cursor alignment and list ordering).

## Setup (Fresh QA Database)

### 1) Create QA config + clean DB

```bash
mkdir -p qa
export BUJO_PATH="$(pwd)/qa/qa.db"
export BUJO_CONFIG_PATH="$(pwd)/qa/.bujo.qa.yaml"

cat > "$BUJO_CONFIG_PATH" <<EOF
path: "$BUJO_PATH"
EOF

rm -rf "$BUJO_PATH"
```

### 2) Seed deterministic QA data

This uses dynamic dates so “today” and “current month” are always relevant.

```bash
eval "$(
python - <<'PY'
import datetime as dt

def fmt(d):
    return d.strftime("%B %d, %Y").replace(" 0", " ")

today = dt.date.today()
yesterday = today - dt.timedelta(days=1)
week_ago = today - dt.timedelta(days=7)
ten_days_ago = today - dt.timedelta(days=10)
tomorrow = today + dt.timedelta(days=1)

next_month = (today.replace(day=1) + dt.timedelta(days=32)).replace(day=1)
prev_month = (today.replace(day=1) - dt.timedelta(days=1)).replace(day=1)
next_month_day = next_month

print(f'export QA_TODAY="{fmt(today)}"')
print(f'export QA_YESTERDAY="{fmt(yesterday)}"')
print(f'export QA_WEEK_AGO="{fmt(week_ago)}"')
print(f'export QA_TEN_DAYS_AGO="{fmt(ten_days_ago)}"')
print(f'export QA_TOMORROW="{fmt(tomorrow)}"')
print(f'export QA_MONTH="{today.strftime("%B %Y")}"')
print(f'export QA_NEXT_MONTH="{next_month.strftime("%B %Y")}"')
print(f'export QA_PREV_MONTH="{prev_month.strftime("%B %Y")}"')
print(f'export QA_NEXT_MONTH_DAY="{fmt(next_month_day)}"')
PY
)"

# Types / parents
go run . collections type "Future" monthly
go run . collections type "$QA_MONTH" daily
go run . collections type "$QA_NEXT_MONTH" daily
go run . collections type "$QA_PREV_MONTH" daily
go run . collections type "Habits" tracking

# Daily collections
go run . add task "QA Today task A" -c "$QA_TODAY"
go run . add task "QA Today task B" -c "$QA_TODAY"
go run . add note "QA Today note" -c "$QA_TODAY"
go run . add event "QA Today event" -c "$QA_TODAY"
go run . add task "QA Yesterday task" -c "$QA_YESTERDAY"
go run . add task "QA Week-ago task (migration)" -c "$QA_WEEK_AGO"
go run . add task "QA Ten-days-ago task (migration)" -c "$QA_TEN_DAYS_AGO"
go run . add task "QA Next-month day task" -c "$QA_NEXT_MONTH_DAY"

# Generic collections
go run . add task "Inbox task 1" -c "Inbox"
go run . add task "Inbox task 2" -c "Inbox"
go run . add note "Project note A" -c "Project Alpha"
go run . add event "Project Alpha kickoff" -c "Project Alpha"

# Tracking
go run . track "Habits"
go run . track "Habits"

# Scroll stress (today)
for i in $(seq 1 15); do
  go run . add task "QA Scroll item $i" -c "$QA_TODAY"
done
```

### 3) Launch the UI

```bash
go run .
```

## Manual Test Checklist

Record results per step. If something fails, capture the observation and steps to reproduce.

### A) Startup / Layout Smoke
- [ ] App launches without errors, shows nav (left) and detail (right), status bar at bottom.
- [ ] Status line shows “Journal loaded”.
- [ ] Resize terminal wider and narrower; layout should adjust (no overlap or truncated bottom bar).

### B) Command Bar + Overlays
- [ ] Press `:` to open command input; suggestions show and update.
- [ ] Use `Tab` / `Shift+Tab` or `Down` / `Up` to move suggestion selection.
- [ ] Press `Enter` to accept a suggestion, and `Esc` to exit command input.
- [ ] `:help` opens overlay; `Esc` closes and focus restores.
- [ ] `:debug` toggles the event viewer; the body should resize and the viewer appears/disappears.
- [ ] `:report 7d` opens report overlay; `Esc` closes it.
- [ ] `:today` and `:future` jump to the expected collection (verify status updates).

### C) Navigation List (Left Pane)
- [ ] Arrow keys / `j`/`k` move selection line-by-line.
- [ ] `Right` / `l` expands a folded node; `Left` / `h` collapses it.
- [ ] Current month appears even if it has only placeholders; today is visible near the top of daily list.
- [ ] Daily collections are sorted with today on top, then prior days (verify order).
- [ ] Selecting a month shows its days; selecting a day updates the detail pane.
- [ ] Rapidly press `Down` 10+ times in nav: detail should update to the corresponding collection each time without jumps or skipped sections.
- [ ] Move `Up` and `Down` across the boundary between months; detail should follow without resetting to an unexpected section.
- [ ] When hovering over a day in the nav (without Enter), detail should preview that day without changing selection in the nav.

### D) Detail Pane (Right Pane)
- [ ] `Tab` moves focus to detail; `Shift+Tab` returns focus to nav.
- [ ] Arrow keys / `j`/`k` move between bullets.
- [ ] `PageUp`/`PageDown` or `b`/`f` scrolls pages without losing selection.
- [ ] `Home`/`g` jumps to top; `End`/`G` jumps to bottom.
- [ ] If a day has no entries (e.g., `$QA_TOMORROW`), detail shows the placeholder message.
- [ ] **Known issue check:** pressing `Down` should not loop to the top or jump to a different day. If it does, record the collection name and steps.
- [ ] With detail focused, press `Down` repeatedly from the first bullet to the last; the cursor should advance sequentially with no skips.
- [ ] After reaching the last bullet, press `Down` once more; selection should stay at the last item (no jump to top).
- [ ] Press `Up` repeatedly from the first bullet; selection should stay at the first item (no wrap).
- [ ] Verify detail section order matches nav order for the current month (same day sequence as nav list).

### E) Add Task Overlay
- [ ] With nav focused on today, press `i` to open the Add Task overlay.
- [ ] Cursor should align with the input field (no offset/ghost cursor). **Known issue check.**
- [ ] `Tab` cycles fields; `Shift+Tab` goes backward.
- [ ] Enter a task message, submit with `Enter`.
- [ ] On return, focus should land on the newly created entry (not a different day). **Known issue check.**
- [ ] Press `i` again, then `Esc` → confirmation prompt; `y` discards and exits.

### F) Bullet Actions (Detail Pane)
- [ ] Select a bullet and press `x` to complete; bullet updates to completed glyph.
- [ ] Select a bullet and press `Backspace` to strike; bullet updates to dropped glyph.
- [ ] Press `*`, `!`, `?` to set signifiers; press `|` to clear.
- [ ] Press `Enter`/`Space` on a bullet to open the bullet detail overlay; verify info renders; `Esc` closes.

### G) Move / Future / New Collection
- [ ] Select a bullet and press `>` to open move overlay.
- [ ] Use nav to pick a different collection and press `Enter`; overlay closes and bullet moves.
- [ ] In move overlay, select “+ New Collection...” and create one; confirm move.
- [ ] Select a bullet and press `<` to move to Future; verify status update and bullet removed from current list.

### H) Migration Overlay
- [ ] Run `:migrate 14d` (or `:migrate 7d`).
- [ ] Use `<` to focus Future nav, `>` to focus target nav; ensure focus indicator changes.
- [ ] Select a candidate and move it to a target; entry disappears from migration list.
- [ ] Select “+ New Collection...” to create a target; verify it is created.
- [ ] `Esc` exits overlay and restores prior focus.

### I) Lock / Unlock
- [ ] Select a bullet and run `:lock`; verify status update.
- [ ] Run `:unlock`; verify status update and bullet becomes movable again.

### J) Quit
- [ ] Run `:quit` to exit cleanly.

## Known Issues to Validate (Explicit Checks)
- [ ] Add Task overlay cursor alignment (should match input field).
- [ ] Down-arrow in detail should not loop to top or change day unexpectedly.
- [ ] Detail list ordering should match nav order for daily collections.

## Notes / Observations

Use this space to log failures, glitches, and reproduction steps:

- …
