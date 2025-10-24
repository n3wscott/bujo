# bujo

`bujo` - Bullet journaling on the command line.

[![GoDoc](https://godoc.org/tableflip.dev/bujo?status.svg)](https://godoc.org/tableflip.dev/bujo)
[![Go Report Card](https://goreportcard.com/badge/n3wscott/bujo)](https://goreportcard.com/report/n3wscott/bujo)

Bullet journaling is a fast way to capture daily tasks, notes, and events. `bujo`
lets you bring that workflow to the terminal with both a classic CLI and a rich
TUI experience.

## Installation

`bujo` can be installed via:

```shell
go install tableflip.dev/bujo@latest
```

  Make sure to update the completion script if you are using the auto-completion:
  ```shell
  . <(bujo completion)
  ```

## Usage
The CLI is a thin layer over your bullet journal. A few common commands:

```shell
# Show the collections in your journal
bujo list

# Add a task into today's daily log
bujo add "Finish README refresh"

# Review recently completed work (defaults to 1 week)
bujo report --last 1w

# Set or create a collection type (monthly/daily/generic/tracking)
bujo collections type "Future" monthly

# Jump into the fullscreen TUI
bujo ui
```

Most users keep the TUI running in a terminal pane (`bujo ui`) and use the CLI
commands from another window to add or migrate tasks. The TUI supports calendar
browsing, bullet/signifier editing, command-mode shortcuts (`:`), and a
scrollable `:report` view to review completed entries within a window. Use
`:type [collection] <type>` to update metadata in place, and launch
`:new-collection` for a wizard that guides you through choosing the parent,
name, and type. New collections created through moves, `:mkdir`, or the wizard
surface the inferred type in their prompts.

## TUI / CLI Delta

As of the latest TUI overhaul there are still some capabilities that exist only
in the CLI or only in the TUI. This checklist will guide future parity work.

**CLI-only today**
- `bujo log` renders day/month/future summaries for piping or quick export; the TUI has no single-shot log output (`pkg/commands/log.go`).
- `bujo track <collection>` appends an occurrence bullet and prints the counter. The TUI bullet picker doesn’t expose the tracking glyph (`pkg/commands/track.go`, `pkg/runner/tea/ui.go:249`).
- `bujo key` emits the bullet/signifier legend; the TUI relies on contextual help and can’t print the cheat sheet (`pkg/commands/key.go`).
- `bujo get …` offers cross-collection filters and `--show-id` output suitable for scripting. The TUI can’t slice entries that way (`pkg/commands/get.go`).
- `bujo report --last` prints reports to stdout for `tee`/redirect; the TUI `:report` overlay is view-only (`pkg/commands/report.go`, `pkg/runner/tea/ui.go:4141`).

**TUI-only today**
- Inline edit/move/future actions via `i`, `>`, `<` and escape hovers have no CLI equivalents (`pkg/runner/tea/ui.go:2520`, `pkg/runner/tea/ui.go:2550`).
- Indent/outdent, parent selection, and hierarchical editing live only behind `tab`, `shift+tab`, and the parent picker (`pkg/runner/tea/ui.go:2695`, `pkg/runner/tea/ui.go:2746`).
- The move selector (tab completion + create-on-enter) and type-aware validation are TUI-only (`pkg/runner/tea/ui.go:1878`).
- Bullet/signifier menus let you change glyphs or defaults after creation; CLI can only set signifiers at add-time (`pkg/runner/tea/ui.go:3226`).
- Overlay workflows—collection wizard, delete confirmation, lock/unlock commands, task metadata panel, and the scrollable report viewport—have no CLI analogs (`pkg/runner/tea/ui.go:2186`, `pkg/runner/tea/ui.go:2963`, `pkg/runner/tea/ui.go:2275`, `pkg/runner/tea/ui.go:4381`).

## Bash Completion

(For Mac)

Make sure you have bash-completion installed:

```shell
brew install bash-completion
```

And make sure the following two lines are in your `.bashrc` or `.profile`:

```text
. /usr/local/etc/profile.d/bash_completion.sh
. <(bujo completion)
```

Now tab completion should work!
