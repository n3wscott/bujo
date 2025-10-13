package commands

import (
	"github.com/spf13/cobra"

	base "github.com/n3wscott/cli-base/pkg/commands/options"
)

var (
	output = &base.OutputOptions{}
)

// New constructs the root bujo Cobra command.
func New() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "bujo",
		Short: base.Wrap80("Bullet journaling on the command line."),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	AddCommands(cmd)
	return cmd
}

// AddCommands attaches all subcommands to the provided root.
func AddCommands(topLevel *cobra.Command) {
	addUI(topLevel)
	addKey(topLevel)
	addAdd(topLevel)
	addGet(topLevel)
	addComplete(topLevel)
	addStrike(topLevel)
	addTrack(topLevel)
	addLog(topLevel)
	addCompletions(topLevel)
	addInfo(topLevel)
	addUpgrade(topLevel)
	addVersion(topLevel)
}
