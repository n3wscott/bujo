package options

import (
	"github.com/spf13/cobra"
	"tableflip.dev/bujo/pkg/glyph"
)

// CollectionOptions
type CollectionOptions struct {
	Bullet     glyph.Bullet
	Collection string
	All        bool
	List       bool
}

func AddCollectionArgs(cmd *cobra.Command, o *CollectionOptions) {
	cmd.Flags().StringVarP(&o.Collection, "collection", "c", "today",
		"Specify the collection.")
}

func AddAllCollectionsArg(cmd *cobra.Command, o *CollectionOptions) {
	cmd.Flags().BoolVar(&o.All, "all", false,
		"Specify all collections.")
	cmd.Flags().BoolVar(&o.List, "list", false,
		"List all collections.")
}
