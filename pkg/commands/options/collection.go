// Package options defines shared flag helpers for CLI commands.
package options

import (
	"github.com/spf13/cobra"
	"tableflip.dev/bujo/pkg/glyph"
)

// CollectionOptions captures common collection selection flags for commands.
type CollectionOptions struct {
	Bullet     glyph.Bullet
	Collection string
	All        bool
	List       bool
}

// AddCollectionArgs wires collection-related flags on the provided command.
func AddCollectionArgs(cmd *cobra.Command, o *CollectionOptions) {
	cmd.Flags().StringVarP(&o.Collection, "collection", "c", "today",
		"Specify the collection.")
}

// AddAllCollectionsArg registers flags that operate on all collections.
func AddAllCollectionsArg(cmd *cobra.Command, o *CollectionOptions) {
	cmd.Flags().BoolVar(&o.All, "all", false,
		"Specify all collections.")
	cmd.Flags().BoolVar(&o.List, "list", false,
		"List all collections.")
}
