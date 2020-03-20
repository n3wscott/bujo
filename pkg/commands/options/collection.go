package options

import (
	"github.com/spf13/cobra"
)

// CollectionOptions
type CollectionOptions struct {
	Collection string
}

func AddCollectionArgs(cmd *cobra.Command, o *CollectionOptions) {
	cmd.Flags().StringVarP(&o.Collection, "collection", "c","today",
		"Specific the collection to add to.")
}
