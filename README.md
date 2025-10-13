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

# Jump into the fullscreen TUI
bujo ui
```

Most users keep the TUI running in a terminal pane (`bujo ui`) and use the CLI
commands from another window to add or migrate tasks. The TUI supports calendar
browsing, bullet/signifier editing, and command-mode shortcuts (`:`).

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
