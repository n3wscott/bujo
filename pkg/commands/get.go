package commands

import (
	"github.com/spf13/cobra"
)

func addGet(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "get something",
		Example: `
bujo get notes
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	getTask(cmd)
	getNote(cmd)

	topLevel.AddCommand(cmd)
}
