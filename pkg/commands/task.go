package commands

import (
	"context"
	"errors"
	"github.com/spf13/cobra"
	"strings"

	"github.com/n3wscott/bujo/pkg/add"
	"github.com/n3wscott/bujo/pkg/commands/options"
	"github.com/n3wscott/bujo/pkg/glyph"
	base "github.com/n3wscott/cli-base/pkg/commands/options"
)

func addTask(topLevel *cobra.Command) {
	no := &options.AddOptions{}
	so := &options.SigOptions{}
	co := &options.CollectionOptions{}

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
			s := add.Add{
				Bullet:        glyph.Task,
				Message:       no.Message,
				Collection:    co.Collection,
				Priority:      so.Priority,
				Inspiration:   so.Inspiration,
				Investigation: so.Investigation,
			}
			err := s.Do(context.Background())
			return oo.HandleError(err)
		},
	}

	options.AddNoteArgs(cmd, no)
	options.AddSigArgs(cmd, so)
	options.AddCollectionArgs(cmd, co)

	base.AddOutputArg(cmd, oo)
	topLevel.AddCommand(cmd)
}
