package options

import (
	"github.com/spf13/cobra"
)

// InteractiveOptions
type InteractiveOptions struct {
	Interactive bool
}

func InteractiveArgs(cmd *cobra.Command, o *InteractiveOptions) {
	cmd.Flags().BoolVarP(&o.Interactive, "interactive", "i", false,
		`Interactive input of subcommands or options.`)
}
