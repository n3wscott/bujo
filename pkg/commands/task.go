package commands

import (
	"context"
	"errors"
	"github.com/n3wscott/bujo/pkg/runner/get"
	"github.com/n3wscott/bujo/pkg/store"
	"github.com/spf13/cobra"
	"strings"

	"github.com/n3wscott/bujo/pkg/commands/options"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/runner/add"
	base "github.com/n3wscott/cli-base/pkg/commands/options"
)

func addTask(topLevel *cobra.Command) {
	no := &options.AddOptions{}
	so := &options.SigOptions{}
	co := &options.CollectionOptions{}

	// TODO: change this to be bujo task add x y z
	// TODO: make a demo of bujo task list

	cmd := &cobra.Command{
		Use:   "task",
		Short: "Add a task",
		Example: `
bujo task do this task
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires a task")
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
				Bullet:        glyph.Task,
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

func getTask(topLevel *cobra.Command) {
	co := &options.CollectionOptions{}

	cmd := &cobra.Command{
		Use:     "task",
		Aliases: []string{"tasks"},
		Short:   "get tasks",
		Example: `
bujo get tasks
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}
			s := get.Get{
				Bullet:      glyph.Task,
				Persistence: p,
				Collection:  co.Collection,
			}
			err = s.Do(context.Background())
			return oo.HandleError(err)
		},
	}

	options.AddCollectionArgs(cmd, co)
	options.AddAllCollectionsArg(cmd, co)

	base.AddOutputArg(cmd, oo)
	topLevel.AddCommand(cmd)
}
