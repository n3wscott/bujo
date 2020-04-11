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
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	AddCommands(cmd)
	return cmd
}

func AddCommands(topLevel *cobra.Command) {
	addUI(topLevel)
	addKey(topLevel)
	addAdd(topLevel)
	addGet(topLevel)
	addComplete(topLevel)
	addStrike(topLevel)
	addTrack(topLevel)

	// Ref: https://bulletjournal.com/pages/learn

	// TODO: Next refactor: make the concept of the daily and the month log more clear.

	// TODO: The UI might be dropped? We can do almost all the things not using it.

	// TODO: make sure the json options works all places.

	// TODO: support nesting - Nesting
	//  this will be interesting, we can add a

	// TODO: look into adding the monthly log, the current impl is the daily log.
	// should add The Task Page ?
	// should add The Calendar Page?

	// --- Planning ---

	// TODO: add migration, this is more than just moving items to the collections, If
	// we had the monthly log, we can auto-migrate things that are still open to the last monthly log and the last daily
	// log into this months monthly log.
	// Likely monthly migration should be fairly mindful and not automatic, maybe in the ui or as a series of interactions
	// like walk each open task and ask if it should be migrated to:
	//  - this month
	//  - future log, then ask for which month.
	//  - strike the task

	// TODO: overall the raw get commands are good but the main UX should be more aligned with how bullet journaling works.
	// maybe main ux is like:
	// bujo log day [a day or a range] [default now]
	// bujo log month [a month] [default this month]
	// bujo log future [a month] [default this month]
	// bujo migrate [no options means migrate all open tasks to this month.]
	// bujo overview - show this months future log, this months log, and ? all days or today ?
	//  - overview could also answer questions like, show me all notes from month x?
	//  - maybe how many tasks were finished? idk...

}
