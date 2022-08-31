package main

import (
	"log"

	"tableflip.dev/bujo/pkg/commands"
)

func main() {
	if err := commands.New().Execute(); err != nil {
		log.Fatalf("error during command execution: %v", err)
	}
}
