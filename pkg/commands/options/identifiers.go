package options

import (
	"github.com/spf13/cobra"
)

// IDOptions manages entry identifier flags.
type IDOptions struct {
	ShowID bool
	ID     string
}

// AddShowIDArgs registers the show-id flag.
func AddShowIDArgs(cmd *cobra.Command, o *IDOptions) {
	cmd.Flags().BoolVarP(&o.ShowID, "show-id", "i", false,
		"Show the ID of the entry or collection.")
}

// AddIDArgs registers the id flag for commands requiring specific entries.
func AddIDArgs(cmd *cobra.Command, o *IDOptions) {
	cmd.Flags().StringVar(&o.ID, "id", "",
		"Specify the id of a entry.")
}
