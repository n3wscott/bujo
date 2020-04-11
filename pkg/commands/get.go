package commands

import (
	"context"
	"errors"
	"fmt"
	"github.com/n3wscott/bujo/pkg/commands/options"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/runner/get"
	"github.com/n3wscott/bujo/pkg/store"
	"github.com/spf13/cobra"
	"strings"
)

func addGet(topLevel *cobra.Command) {
	co := &options.CollectionOptions{}
	io := &options.IDOptions{}

	long := strings.Builder{}
	long.WriteString("Get all or a filtered set of bullets.\n\n")
	long.WriteString("Bullet and aliases:\n")
	for _, g := range glyph.DefaultBullets() {
		if g.Symbol == "" {
			continue
		}

		long.WriteString(fmt.Sprintf("%s: %s\n", g.Symbol, strings.Join(g.Aliases, ", ")))
	}

	cmd := &cobra.Command{
		Use:   "get [bullet]",
		Short: "get something",
		Long:  long.String(),
		Example: `
bujo get notes
bujo get tasks --collection future
bujo get completed --all
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				co.Bullet = glyph.Any
				return nil
			}
			var err error
			co.Bullet, err = glyph.BulletForAlias(args[0])

			if len(args) > 1 {
				if co.Collection != "today" {
					return errors.New("too many collections set, confused")
				}
				co.Collection = strings.Join(args[1:], " ")
			}

			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			s := get.Get{
				ShowID:          io.ShowID,
				Bullet:          co.Bullet,
				Persistence:     p,
				Collection:      co.Collection,
				ListCollections: co.List,
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
	options.AddShowIDArgs(cmd, io)

	topLevel.AddCommand(cmd)
}
