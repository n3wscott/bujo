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

The CLI mirrors common bullet journal workflows:

```shell
# open the text UI
bujo ui

# add a task, note, or event
bujo add task "review pull request"
bujo add note "ideas for next sprint"

# list a collection
bujo get --collection "October 15, 2025"

# mark items complete or irrelevant
bujo complete <entry-id>
bujo strike <entry-id>
```

Run `bujo --help` for a complete command list.

## MCP Integration

`bujo` ships with a Model Context Protocol (MCP) server so LLM clients such as ChatGPT, OpenAI Studio, or the Codex CLI can call the journal programmatically.

```shell
# start an MCP server over stdio
bujo mcp
```

The server exposes:

- **Resources** – list collections (`bujo://collections`), inspect a collection (`bujo://collections/{name}`), or fetch an entry (`bujo://entries/{id}`).
- **Tools** – create entries, complete/strike items, move bullets, update signifiers, list or search entries.

Point your MCP-capable client at the executable and the server will handle authentication-free stdio transport using [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go).

Most users keep the TUI running in a terminal pane (`bujo ui`) and use CLI commands from another window to add or migrate tasks. The TUI supports calendar browsing, bullet/signifier editing, and command-mode shortcuts (`:`).

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
