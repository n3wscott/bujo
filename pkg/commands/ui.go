package commands

import (
	"context"
	"tableflip.dev/bujo/pkg/store"

	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/runner/ui"
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
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			i := ui.UI{Persistence: p}
			return i.Do(context.Background())
		},
	}

	topLevel.AddCommand(cmd)
}
