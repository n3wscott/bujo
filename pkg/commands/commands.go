package commands

import (
	"github.com/spf13/cobra"

	base "github.com/n3wscott/cli-base/pkg/commands/options"

	appsvc "tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/store"
	newtui "tableflip.dev/bujo/pkg/tui/app"
)

var (
	output = &base.OutputOptions{}
)

// New constructs the root bujo Cobra command.
func New() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "bujo",
		Short: base.Wrap80("Bullet journaling on the command line."),
		RunE: func(_ *cobra.Command, _ []string) error {
			persistence, err := store.Load(nil)
			if err != nil {
				return err
			}
			service := &appsvc.Service{Persistence: persistence}
			return newtui.Run(service)
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
	addCollections(topLevel)
	addComplete(topLevel)
	addStrike(topLevel)
	addReport(topLevel)
	addMigration(topLevel)
	addTrack(topLevel)
	addLog(topLevel)
	addCompletions(topLevel)
	addInfo(topLevel)
	addUpgrade(topLevel)
	addVersion(topLevel)
}
