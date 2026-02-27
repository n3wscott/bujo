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
# Add a task into today's daily log
bujo add task "Finish README refresh"

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

### Machine Contract (`bujo api`)

Machine-oriented automation should use `bujo api ...` commands. This namespace
is JSON-first, supports explicit `--journal` selection, and provides structured
success/error envelopes. Legacy human CLI commands remain unchanged.

```shell
bujo api info --journal /tmp/codex-bujo.db
bujo api entries add --journal /tmp/codex-bujo.db --type task --message "Follow up"
bujo api entries list --journal /tmp/codex-bujo.db --collection today
bujo api entries complete --journal /tmp/codex-bujo.db --id <entry-id>
bujo api collections ensure --journal /tmp/codex-bujo.db --name project/alpha
```

### Labels in `bujo api`

`bujo api` supports loose, queryable labels on entries.

Normalization rules:
- Labels are trimmed.
- Labels are lowercased.
- Duplicate labels are removed.
- Labels are sorted for stable output.

Filter semantics:
- `--label` is an AND filter. Repeating `--label` requires all listed labels.
- `--label-any` is an OR filter. At least one listed label must match.
- `--without-label` excludes entries that contain any listed label.

Recommended naming conventions:
- `owner:<actor>` for assignment (`owner:codex`, `owner:snichols`)
- `area:<domain>` for scope (`area:api`, `area:docs`, `area:infra`)
- `state:<status>` for workflow tags (`state:open`, `state:blocked`, `state:done`)

Examples:

```shell
# Add labels at create time
bujo api entries add --journal /tmp/codex-bujo.db \
  --type task \
  --message "Implement auth middleware" \
  --label owner:codex \
  --label area:api \
  --label state:open

# AND semantics: must have both labels
bujo api entries list --journal /tmp/codex-bujo.db --all \
  --label owner:codex \
  --label area:api

# OR semantics: any owner match
bujo api entries list --journal /tmp/codex-bujo.db --all \
  --label-any owner:codex \
  --label-any owner:snichols

# Exclusion filter
bujo api entries list --journal /tmp/codex-bujo.db --all \
  --without-label state:done

# Mutate labels after creation
bujo api entries labels add --journal /tmp/codex-bujo.db --id <entry-id> --label state:blocked
bujo api entries labels remove --journal /tmp/codex-bujo.db --id <entry-id> --label state:blocked
bujo api entries labels set --journal /tmp/codex-bujo.db --id <entry-id> --label state:open
bujo api entries labels clear --journal /tmp/codex-bujo.db --id <entry-id>
```

### Command Tree

<!-- BEGIN GENERATED COMMANDS -->

Generated from the Cobra command tree (`go run ./cmd/gendocs --check`).

```text
bujo - Bullet journaling on the command line.
  bujo add - Add something
    bujo add event - Add an event
    bujo add note - Add a note
    bujo add task - Add a task
    bujo add track - track something
  bujo api - Machine-oriented API commands
    bujo api collections - Machine-oriented collection operations
      bujo api collections ensure - Ensure a collection exists
    bujo api entries - Machine-oriented entry operations
      bujo api entries add - Create a new entry
      bujo api entries complete - Mutate an entry: complete
      bujo api entries delete - Mutate an entry: delete
      bujo api entries labels - Manage entry labels
        bujo api entries labels add - Mutate entry labels: add
        bujo api entries labels clear - Mutate an entry: clear
        bujo api entries labels remove - Mutate entry labels: remove
        bujo api entries labels set - Mutate entry labels: set
      bujo api entries list - List entries
      bujo api entries lock - Mutate an entry: lock
      bujo api entries move - Move an entry to another collection
      bujo api entries parent - Manage entry parent relationships
        bujo api entries parent set - Set an entry parent
        bujo api entries parent unset - Remove an entry parent
      bujo api entries resolve - Resolve a selector to one entry
      bujo api entries strike - Mutate an entry: strike
      bujo api entries unlock - Mutate an entry: unlock
    bujo api info - Show resolved runtime configuration
    bujo api report - Return completed-entry report data
  bujo collections - Manage collections and metadata
    bujo collections type - Set the type for a collection
  bujo complete - complete something
  bujo completion - Generates bash completion scripts
  bujo get - get something
  bujo info - Details about collection and where they are stored.
  bujo key - Print the bullets and signifiers
  bujo log - view a log
  bujo migration - Inspect tasks eligible for migration
    bujo migration list - List migration candidates using the specified time window
  bujo report - Display recently completed entries grouped by collection
  bujo strike - mark something irrelevant
  bujo track - track something
  bujo ui - open the text-based user interface
  bujo upgrade - Upgrade bujo cli.
  bujo version - Get bujo version.
```

<!-- END GENERATED COMMANDS -->

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
