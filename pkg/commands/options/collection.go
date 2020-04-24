package options

import (
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// CollectionOptions
type CollectionOptions struct {
	Bullet     glyph.Bullet
	Collection string
	All        bool
	List       bool
}

func AddCollectionArgs(cmd *cobra.Command, o *CollectionOptions) {
	flag := &pflag.Flag{
		Name:      "collection",
		Shorthand: "c",
		Usage:     "Specify the collection.",
		DefValue:  "today",
	}
	cmd.Flags().AddFlag(flag)
}

func AddAllCollectionsArg(cmd *cobra.Command, o *CollectionOptions) {
	cmd.Flags().BoolVar(&o.All, "all", false,
		"Specify all collections.")
	cmd.Flags().BoolVar(&o.List, "list", false,
		"List all collections.")
}
