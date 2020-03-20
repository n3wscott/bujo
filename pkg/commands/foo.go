package commands

import (
	"context"
	"github.com/spf13/cobra"

	"github.com/n3wscott/cli-base/pkg/commands/options"
	"github.com/n3wscott/cli-base/pkg/foo"
)

func addFoo(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:       "foo",
		Short:     options.Wrap80("Foo a thing."),
		ValidArgs: []string{},
		Run: func(cmd *cobra.Command, args []string) {
			// a sub-command is required.
			_ = cmd.Help()
		},
	}

	addFooGet(cmd)

	topLevel.AddCommand(cmd)
}

func addFooGet(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:       "list",
		ValidArgs: []string{},
		Short:     "Get a list of foo.",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := foo.Foo{
				List: true,
			}
			if oo.JSON {
				c.Output = "json"
			}
			err := c.Do(context.Background())
			return oo.HandleError(err)
		},
	}
	options.AddOutputArg(cmd, oo)

	topLevel.AddCommand(cmd)
}
