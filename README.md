# bujo

`bujo` - Bullet journaling on the command line. 

[![GoDoc](https://godoc.org/github.com/n3wscott/bujo?status.svg)](https://godoc.org/github.com/n3wscott/bujo)
[![Go Report Card](https://goreportcard.com/badge/n3wscott/bujo)](https://goreportcard.com/report/n3wscott/bujo)

_Work in progress._

## Installation

`bujo` can be installed via:

```shell
go get github.com/n3wscott/bujo/cmd/bujo
```

To update your installation:

```shell
go get -u github.com/n3wscott/bujo/cmd/bujo
```

Or to install/update from source:

```shell
go install ./cmd/bujo/
```

  Make sure to update the completion script if you are using the auto-completion:
  ```shell
  . <(bujo completion)
  ```

## Usage

TBD

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
