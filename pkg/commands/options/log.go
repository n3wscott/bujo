package options

import (
	"github.com/spf13/cobra"
)

// LogOptions
type LogOptions struct {
	Day    bool
	Month  bool
	Future bool
}

func AddLogArgs(cmd *cobra.Command, o *LogOptions) {
	cmd.Flags().BoolVarP(&o.Day, "day", "d", false,
		"Show day log.")
	cmd.Flags().BoolVarP(&o.Month, "month", "m", false,
		"Show month log.")
	cmd.Flags().BoolVarP(&o.Future, "future", "f", false,
		"Show future log.")
}
