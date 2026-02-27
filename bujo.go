// bujo is the CLI entrypoint for the bullet journal application.
package main

import (
	"os"

	"tableflip.dev/bujo/pkg/commands"
)

func main() {
	if err := commands.New().Execute(); err != nil {
		os.Exit(1)
	}
}
