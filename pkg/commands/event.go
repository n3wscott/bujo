package commands

import (
	"context"
	"errors"
	"strings"

	"github.com/n3wscott/bujo/pkg/commands/options"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/runner/add"
	"github.com/n3wscott/bujo/pkg/store"
	base "github.com/n3wscott/cli-base/pkg/commands/options"
	"github.com/spf13/cobra"
)

func addEvent(topLevel *cobra.Command) {
	no := &options.AddOptions{}
	oo := &options.OnOptions{}
	so := &options.SigOptions{}
	co := &options.CollectionOptions{}

	cmd := &cobra.Command{
		Use:   "event",
		Short: "Add an event",
		Example: `
bujo add event a fun party --on=1999-12-31
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires an event title")
			}
			no.Message = strings.Join(args, " ")

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}

			on, err := oo.GetOn()
			if err != nil {
				return err
			}

			s := add.Add{
				Bullet:        glyph.Event,
				Persistence:   p,
				Message:       no.Message,
				Collection:    co.Collection,
				Priority:      so.Priority,
				Inspiration:   so.Inspiration,
				Investigation: so.Investigation,
				On:            on,
			}
			err = s.Do(context.Background())
			return output.HandleError(err)
		},
	}

	options.AddOnArgs(cmd, oo)
	options.AddSigArgs(cmd, so)
	options.AddCollectionArgs(cmd, co)

	base.AddOutputArg(cmd, output)
	topLevel.AddCommand(cmd)
}
