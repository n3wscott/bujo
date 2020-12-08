package commands

import (
	"github.com/n3wscott/bujo/pkg/snake"
	"github.com/spf13/cobra"
)

func addAdd(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add something",
		Example: `
bujo add note this is a note
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return snake.PromptNext(cmd, args)
		},
	}

	addTask(cmd)
	addNote(cmd)
	addEvent(cmd)
	addTrack(cmd)

	topLevel.AddCommand(cmd)
}
