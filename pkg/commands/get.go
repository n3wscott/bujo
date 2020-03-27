package commands

import (
	"context"
	"github.com/n3wscott/bujo/pkg/commands/options"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/runner/get"
	"github.com/n3wscott/bujo/pkg/store"
	"github.com/spf13/cobra"
)

func addGet(topLevel *cobra.Command) {
	co := &options.CollectionOptions{}
	io := &options.IDOptions{}

	cmd := &cobra.Command{
		Use:   "get",
		Short: "get something",
		Example: `
bujo get notes
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			s := get.Get{
				ShowID:      io.ShowID,
				Bullet:      glyph.Any,
				Persistence: p,
				Collection:  co.Collection,
			}
			if co.All {
				s.Collection = ""
			}
			err = s.Do(context.Background())
			return oo.HandleError(err)
		},
	}

	options.AddCollectionArgs(cmd, co)
	options.AddAllCollectionsArg(cmd, co)
	options.AddIDArgs(cmd, io)

	getTask(cmd)
	getNote(cmd)

	topLevel.AddCommand(cmd)
}
