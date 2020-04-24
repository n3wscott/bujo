package commands

import (
	"context"
	"github.com/n3wscott/bujo/pkg/runner/info"
	"github.com/n3wscott/bujo/pkg/store"
	"github.com/spf13/cobra"
)

func addInfo(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Details about collection and where they are stored.",
		Example: `
bujo info
`,
		RunE: func(cmd *cobra.Command, args []string) error {
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
