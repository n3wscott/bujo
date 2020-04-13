package commands

import (
	"context"
	"github.com/n3wscott/bujo/pkg/commands/options"
	"github.com/n3wscott/bujo/pkg/runner/log"
	"github.com/n3wscott/bujo/pkg/store"
	"github.com/spf13/cobra"
)

func addLog(topLevel *cobra.Command) {
	lo := &options.LogOptions{}

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
			s := log.Log{
				Persistence: p,
				Day:         lo.Day,
				Month:       lo.Month,
				Future:      lo.Future,
				On:          lo.On,
			}
			err = s.Do(context.Background())
			return oo.HandleError(err)
		},
	}

	options.AddLogArgs(cmd, lo)

	topLevel.AddCommand(cmd)
}
