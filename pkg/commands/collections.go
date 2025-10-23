package commands

import (
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/runner/collections"
	"tableflip.dev/bujo/pkg/store"
)

func addCollections(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "collections",
		Short: "Manage collections and metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newCollectionsTypeCmd())
	topLevel.AddCommand(cmd)
}

func newCollectionsTypeCmd() *cobra.Command {
	var create bool
	typeCmd := &cobra.Command{
		Use:   "type <collection> <type>",
		Short: "Set the type for a collection",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			rawType := args[1]
			typ, err := collection.ParseType(rawType)
			if err != nil {
				return err
			}

			persist, err := store.Load(nil)
			if err != nil {
				return err
			}

			r := collections.Type{
				Collection:  name,
				Type:        typ,
				Create:      create,
				Persistence: persist,
			}
			return r.Do(cmd.Context())
		},
	}
	typeCmd.Flags().BoolVar(&create, "create", true, "create the collection if it is missing")
	return typeCmd
}
