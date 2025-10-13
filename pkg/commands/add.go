// Package commands wires up the CLI command tree.
package commands

import (
	"github.com/spf13/cobra"
)

func addAdd(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add something",
		Example: `
bujo add note this is a note
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	addTask(cmd)
	addNote(cmd)
	addEvent(cmd)
	addTrack(cmd)

	topLevel.AddCommand(cmd)
}
