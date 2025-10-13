package commands

import (
	"context"
	"errors"
	"github.com/spf13/cobra"
	"strings"
	"tableflip.dev/bujo/pkg/commands/options"
	"tableflip.dev/bujo/pkg/runner/complete"
	"tableflip.dev/bujo/pkg/store"
)

func addComplete(topLevel *cobra.Command) {
	io := &options.IDOptions{}

	cmd := &cobra.Command{
		Use:     "complete",
		Aliases: []string{"completed", "done"},
		Short:   "complete something",
		Example: `
bujo complete <entry id>
`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires a entry id")
			}
			io.ID = strings.Join(args, " ")

			return nil
		},

		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			s := complete.Complete{
				ID:          io.ID,
				Persistence: p,
			}
			err = s.Do(context.Background())
			return output.HandleError(err)
		},
	}

	topLevel.AddCommand(cmd)
}
