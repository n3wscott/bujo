package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"tableflip.dev/bujo/pkg/commands"
)

func main() {
	var (
		checkOnly bool
		write     bool
		readme    string
	)
	flag.BoolVar(&checkOnly, "check", false, "Check README generated commands are up to date")
	flag.BoolVar(&write, "write", false, "Write generated command section into README")
	flag.StringVar(&readme, "readme", "README.md", "Path to README")
	flag.Parse()

	if checkOnly && write {
		fmt.Fprintln(os.Stderr, "use either --check or --write")
		os.Exit(2)
	}

	abs, err := filepath.Abs(readme)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve readme path: %v\n", err)
		os.Exit(1)
	}

	current, err := os.ReadFile(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", abs, err)
		os.Exit(1)
	}

	next, err := commands.ReplaceGeneratedCommandsSection(string(current))
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate section: %v\n", err)
		os.Exit(1)
	}

	same := string(current) == next
	if checkOnly {
		if !same {
			fmt.Fprintln(os.Stderr, "README command section is out of date; run: go run ./cmd/gendocs --write")
			os.Exit(1)
		}
		return
	}

	if write {
		if err := os.WriteFile(abs, []byte(next), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", abs, err)
			os.Exit(1)
		}
		return
	}

	if !same {
		_, _ = fmt.Fprint(os.Stdout, next)
		return
	}
	_, _ = fmt.Fprint(os.Stdout, string(current))
}
