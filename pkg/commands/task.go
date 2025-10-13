package commands

import (
	"context"
	"errors"
	"strings"

	base "github.com/n3wscott/cli-base/pkg/commands/options"
	"github.com/spf13/cobra"
	"tableflip.dev/bujo/pkg/commands/options"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/runner/add"
	"tableflip.dev/bujo/pkg/store"
)

func addTask(topLevel *cobra.Command) {
	no := &options.AddOptions{}
	so := &options.SigOptions{}
	co := &options.CollectionOptions{}

	cmd := &cobra.Command{
		Use:   "task",
		Short: "Add a task",
		Example: `
bujo add task do this task
`,
		Args: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if len(args) < 1 {
				return errors.New("requires a task")
			}
			no.Message = strings.Join(args, " ")

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
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
			return output.HandleError(err)
		},
	}

	options.AddSigArgs(cmd, so)
	options.AddCollectionArgs(cmd, co)

	flagName := "collection"
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return collectionCompletions(toComplete), cobra.ShellCompDirectiveNoFileComp
	})

	base.AddOutputArg(cmd, output)
	topLevel.AddCommand(cmd)
}
