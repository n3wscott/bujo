package commands

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/collection"
)

func addAPICollections(topLevel *cobra.Command, opts *apiOptions) {
	cmd := &cobra.Command{
		Use:   "collections",
		Short: "Machine-oriented collection operations",
	}

	addAPICollectionsEnsure(cmd, opts)

	topLevel.AddCommand(cmd)
}

func addAPICollectionsEnsure(topLevel *cobra.Command, opts *apiOptions) {
	var (
		name string
		typ  string
	)

	cmd := &cobra.Command{
		Use:   "ensure",
		Short: "Ensure a collection exists",
		RunE: func(cmd *cobra.Command, args []string) error {
			collectionName := strings.TrimSpace(name)
			if collectionName == "" {
				collectionName = strings.TrimSpace(strings.Join(args, " "))
			}
			if collectionName == "" {
				return apiFailure(cmd, opts, "collections.ensure", expandUserPath(opts.Journal), newAPIError("invalid_argument", "collection name is required"))
			}

			return runAPI(cmd, opts, "collections.ensure", func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				requestedType := strings.TrimSpace(typ)
				if requestedType == "" {
					if err := runtime.Service.EnsureCollection(ctx, collectionName); err != nil {
						return nil, err
					}
				} else {
					parsedType, err := collection.ParseType(requestedType)
					if err != nil {
						return nil, newAPIError("invalid_argument", err.Error())
					}
					if err := runtime.Service.EnsureCollectionOfType(ctx, collectionName, parsedType); err != nil {
						return nil, err
					}
				}

				metaType := string(collection.TypeGeneric)
				metas, err := runtime.Service.CollectionsMeta(ctx, "")
				if err != nil {
					return nil, err
				}
				for _, meta := range metas {
					if meta.Name == collectionName {
						if meta.Type != "" {
							metaType = string(meta.Type)
						}
						break
					}
				}

				return map[string]any{
					"collection": map[string]string{
						"name": collectionName,
						"type": metaType,
					},
				}, nil
			})
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Collection path")
	cmd.Flags().StringVar(&typ, "type", "", "Collection type (generic, monthly, daily, tracking)")
	topLevel.AddCommand(cmd)
}
