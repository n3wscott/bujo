package options

import (
	"github.com/spf13/cobra"
)

// CollectionOptions
type CollectionOptions struct {
	Collection string
	All        bool
}

func AddCollectionArgs(cmd *cobra.Command, o *CollectionOptions) {
	cmd.Flags().StringVarP(&o.Collection, "collection", "c", "today",
		"Specific the collection, defaults to today.")
}

func AddAllCollectionsArg(cmd *cobra.Command, o *CollectionOptions) {
	cmd.Flags().BoolVar(&o.All, "all", false,
		"Specific all collections.")
}
