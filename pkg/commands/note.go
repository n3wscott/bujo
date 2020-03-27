package commands

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/n3wscott/bujo/pkg/commands/options"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/runner/add"
	"github.com/n3wscott/bujo/pkg/runner/get"
	"github.com/n3wscott/bujo/pkg/store"
	base "github.com/n3wscott/cli-base/pkg/commands/options"
)

func addNote(topLevel *cobra.Command) {
	no := &options.AddOptions{}
	so := &options.SigOptions{}
	co := &options.CollectionOptions{}

	// TODO: change this to be bujo note add x y z
	// TODO: make a demo of bujo task list

	cmd := &cobra.Command{
		Use:     "note",
		Aliases: []string{"notes"},
		Short:   "Add a note",
		Example: `
bujo note this is a note
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires a note")
			}
			no.Message = strings.Join(args, " ")

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			s := add.Add{
				Bullet:        glyph.Note,
				Persistence:   p,
				Message:       no.Message,
				Collection:    co.Collection,
				Priority:      so.Priority,
				Inspiration:   so.Inspiration,
				Investigation: so.Investigation,
			}
			err = s.Do(context.Background())
			return oo.HandleError(err)
		},
	}

	options.AddNoteArgs(cmd, no)
	options.AddSigArgs(cmd, so)
	options.AddCollectionArgs(cmd, co)

	base.AddOutputArg(cmd, oo)
	topLevel.AddCommand(cmd)
}

func getNote(topLevel *cobra.Command) {
	co := &options.CollectionOptions{}
	io := &options.IDOptions{}

	cmd := &cobra.Command{
		Use:     "note",
		Aliases: []string{"notes"},
		Short:   "get notes",
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
				Bullet:      glyph.Note,
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

	base.AddOutputArg(cmd, oo)
	topLevel.AddCommand(cmd)
}
