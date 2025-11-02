# Bullet Journal

bujo keeps your bullet journal close to the command line—capture tasks, notes, and events without leaving the terminal.

## Bullets

| Bullet | Meaning |
| --- | --- |
| ⦁ | task |
| ✘ | task completed |
| › | task moved to another collection |
| ‹ | task migrated to the future log |
| ⦵ | task dropped as irrelevant |
| ⁃ | note |
| ○ | event |

## Signifiers

| Signifier | Meaning |
| --- | --- |
| ✷ | priority |
| ! | inspiration |
| ? | investigation |

## Commands

| Command | Description |
| --- | --- |
| `:help` | Open this reference overlay. |
| `:report [window]` | Show completed entries for the given window (`1w`, `3d`, etc.). |
| `:debug` | Toggle the event log at the bottom of the screen. |
| `:quit`, `:exit`, `:q` | Leave the TUI. |

### Navigation & Editing

- Arrow keys / `j` `k` move through collections and bullets.
- `i` opens the add-task overlay for the focused collection or bullet.
- `Esc` cancels overlays or prompts; `:` enters command mode from the status bar.
- Within this help overlay use the arrow keys, PageUp/PageDown, or mouse wheel to scroll. Press `Esc` or `:` to close.
