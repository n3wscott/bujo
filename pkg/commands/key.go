package commands

import (
	"context"
	"github.com/spf13/cobra"

	"github.com/n3wscott/bujo/pkg/runner/key"
)

func addKey(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Print the bullets and signifiers",
		Example: `
bujo key
`,
		ValidArgs: []string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			k := key.Key{}
			err := k.Do(context.Background())
			return output.HandleError(err)
		},
	}

	topLevel.AddCommand(cmd)
}
