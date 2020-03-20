# cli-base

`cli-base` is template for CLI generation.

[![GoDoc](https://godoc.org/github.com/n3wscott/cli-base?status.svg)](https://godoc.org/github.com/n3wscott/cli-base)
[![Go Report Card](https://goreportcard.com/badge/n3wscott/cli-base)](https://goreportcard.com/report/n3wscott/cli-base)

_Work in progress._

## Installation

`cli-base` can be installed via:

```shell
go get github.com/n3wscott/cli-base/cmd/cli
```

To update your installation:

```shell
go get -u github.com/n3wscott/cli-base/cmd/cli
```

## Usage

`cli` has two commands, `foo` and `bar`

```shell
Interact via the command line.

Usage:
  cli [command]

Available Commands:
  bar         Say hello!
  foo         Foo a thing.
  help        Help about any command

Flags:
  -h, --help   help for cli

Use "cli [command] --help" for more information about a command.
```

### Foo

```shell
Foo a thing.

Usage:
  cli foo [flags]
  cli foo [command]

Available Commands:
  list        Get a list of foo.

Flags:
  -h, --help   help for foo

Use "cli foo [command] --help" for more information about a command.
```


### Bar

```shell
Say hello!

Usage:
  cli bar [flags]

Examples:

cli-base bar hello --name=example


Flags:
  -h, --help          help for bar
      --json          Output as JSON.
      --name string   Bar Name to use. (default "World!")
```