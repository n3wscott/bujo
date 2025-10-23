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
`:type [collection] <type>` to update metadata in place; new collections created
through moves or `:mkdir` surface the inferred type in their prompts.

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
