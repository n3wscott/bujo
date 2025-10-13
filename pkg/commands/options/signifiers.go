package options

import (
	"github.com/spf13/cobra"
)

// SigOptions configures signifier-related flags.
type SigOptions struct {
	Priority      bool
	Inspiration   bool
	Investigation bool
}

// AddSigArgs registers signifier flags against the command.
func AddSigArgs(cmd *cobra.Command, o *SigOptions) {
	cmd.Flags().BoolVarP(&o.Priority, "priority", "*", false,
		"Set a priority signifier.")
	cmd.Flags().BoolVarP(&o.Inspiration, "inspiration", "!", false,
		"Set a inspiration signifier.")
	cmd.Flags().BoolVarP(&o.Investigation, "investigation", "?", false,
		"Set a investigation signifier.")
}
