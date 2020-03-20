package commands

import (
	"github.com/n3wscott/cli-base/pkg/commands/options"
	"github.com/spf13/cobra"
)

var (
	oo = &options.OutputOptions{}
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cli",
		Short: options.Wrap80("Interact via the command line."),
	}

	AddCommands(cmd)
	return cmd
}

func AddCommands(topLevel *cobra.Command) {
	addFoo(topLevel)
	addBar(topLevel)
}
