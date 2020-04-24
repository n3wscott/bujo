package commands

import (
	"context"
	"github.com/n3wscott/bujo/pkg/store"
	"github.com/spf13/cobra"
	"os"
	"strconv"
)

func addCompletions(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generates bash completion scripts",
		Long: `To load completion run

. <(bujo completion)

To configure your bash shell to load completions for each session add to your bashrc

# ~/.bashrc or ~/.profile
. <(bujo completion)
`,
		Run: func(cmd *cobra.Command, args []string) {
			_ = topLevel.GenBashCompletion(os.Stdout)
		},
	}

	topLevel.AddCommand(cmd)
}

func collectionCompletions(toComplete string) []string {
	p, err := store.Load(nil)
	if err != nil {
		return nil
	}
	cs := p.Collections(context.Background(), toComplete)
	for i, _ := range cs {
		cs[i] = strconv.Quote(cs[i])
	}
	return cs
}
