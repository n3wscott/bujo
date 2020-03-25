package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/n3wscott/bujo/pkg/runner/ui"
)

func addUI(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "open the text-based user interface",
		Example: `
bujo ui
`,
		ValidArgs: []string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			return ui.Do(context.Background(), ui.StaticDemo()...)
		},
	}

	topLevel.AddCommand(cmd)
}
