package commands

import (
	"github.com/spf13/cobra"

	base "github.com/n3wscott/cli-base/pkg/commands/options"
)

var (
	oo = &base.OutputOptions{}
)

func New() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "bujo",
		Short: base.Wrap80("Bullet journaling on the command line."),
	}

	AddCommands(cmd)
	return cmd
}

func AddCommands(topLevel *cobra.Command) {
	addKey(topLevel)
	addTask(topLevel)
	addNote(topLevel)
}
