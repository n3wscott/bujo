package commands

import (
	"context"
	"github.com/spf13/cobra"
	"tableflip.dev/bujo/pkg/runner/info"
	"tableflip.dev/bujo/pkg/store"
)

func addInfo(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Details about collection and where they are stored.",
		Example: `
bujo info
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			s := info.Info{
				Config:      nil,
				Persistence: p,
			}
			err = s.Do(context.Background())
			return output.HandleError(err)
		},
	}

	topLevel.AddCommand(cmd)
}
