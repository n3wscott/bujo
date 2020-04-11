package commands

import (
	"context"
	"errors"
	"github.com/n3wscott/bujo/pkg/commands/options"
	"github.com/n3wscott/bujo/pkg/runner/strike"
	"github.com/n3wscott/bujo/pkg/store"
	"github.com/spf13/cobra"
	"strings"
)

func addStrike(topLevel *cobra.Command) {
	io := &options.IDOptions{}

	cmd := &cobra.Command{
		Use:     "strike",
		Aliases: []string{"irrelevant"},
		Short:   "mark something irrelevant something",
		Example: `
bujo strike <entry id>
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires a entry id")
			}
			io.ID = strings.Join(args, " ")

			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			s := strike.Strike{
				ID:          io.ID,
				Persistence: p,
			}
			err = s.Do(context.Background())
			return oo.HandleError(err)
		},
	}

	topLevel.AddCommand(cmd)
}
