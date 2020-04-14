package commands

import (
	"context"
	"github.com/n3wscott/bujo/pkg/commands/options"
	"github.com/n3wscott/bujo/pkg/runner/log"
	"github.com/n3wscott/bujo/pkg/store"
	"github.com/spf13/cobra"
	"time"
)

func addLog(topLevel *cobra.Command) {
	lo := &options.LogOptions{}
	oo := &options.OnOptions{}

	cmd := &cobra.Command{
		Use:   "log",
		Short: "view a log",
		Example: `
bujo log --day
bujo log --month
bujo log --future
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := store.Load(nil)
			if err != nil {
				return err
			}

			on, err := oo.GetOn()
			if err != nil {
				return err
			}
			if on == nil {
				now := time.Now()
				on = &now
			}

			s := log.Log{
				Persistence: p,
				Day:         lo.Day,
				Month:       lo.Month,
				Future:      lo.Future,
				On:          *on,
			}
			err = s.Do(context.Background())
			return output.HandleError(err)
		},
	}

	options.AddLogArgs(cmd, lo)
	options.AddOnArgs(cmd, oo)

	topLevel.AddCommand(cmd)
}
