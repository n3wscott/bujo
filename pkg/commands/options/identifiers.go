package options

import (
	"github.com/spf13/cobra"
)

// IDOptions
type IDOptions struct {
	ShowID bool
	ID     string
}

func AddShowIDArgs(cmd *cobra.Command, o *IDOptions) {
	cmd.Flags().BoolVarP(&o.ShowID, "show-id", "k", false,
		"Show the ID of the entry or collection.")
}

func AddIDArgs(cmd *cobra.Command, o *IDOptions) {
	cmd.Flags().StringVar(&o.ID, "id", "",
		"Specify the id of a entry.")
}
