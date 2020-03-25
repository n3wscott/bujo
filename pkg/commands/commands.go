package commands

import (
	"context"
	"github.com/n3wscott/bujo/pkg/ui"
	"github.com/spf13/cobra"

	base "github.com/n3wscott/cli-base/pkg/commands/options"
)

var (
	oo = &base.OutputOptions{}
)

func New() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "bujo",
		Short: base.Wrap80("Bullet journaling on the command line. No arguments gives the TUI."),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ui.Do(context.Background(), ui.StaticDemo()...)
		},
	}

	AddCommands(cmd)
	return cmd
}

func AddCommands(topLevel *cobra.Command) {
	addKey(topLevel)
	addAdd(topLevel)
}
