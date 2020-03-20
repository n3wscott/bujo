package options

import (
	"github.com/spf13/cobra"
)

// BarOptions
type BarOptions struct {
	Message string
	Name    string
}

func AddBarArgs(cmd *cobra.Command, o *BarOptions) {
	cmd.Flags().StringVar(&o.Name, "name", "World!",
		Wrap80("Bar Name to use."))
}
