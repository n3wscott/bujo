package commands

import (
	"context"
	"errors"
	"github.com/spf13/cobra"
	"strings"
	"tableflip.dev/bujo/pkg/commands/options"
	"tableflip.dev/bujo/pkg/runner/track"
	"tableflip.dev/bujo/pkg/store"
)

func addTrack(topLevel *cobra.Command) {
	co := &options.CollectionOptions{}

	cmd := &cobra.Command{
		Use:   "track",
		Short: "track something",
		Example: `
bujo track <thing>
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires a collection")
			}
			co.Collection = strings.Join(args, " ")

			return nil
		},

		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return collectionCompletions(toComplete), cobra.ShellCompDirectiveNoFileComp
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			s := track.Track{
				Collection:  co.Collection,
				Persistence: p,
			}
			err = s.Do(context.Background())
			return output.HandleError(err)
		},
	}

	topLevel.AddCommand(cmd)
}
