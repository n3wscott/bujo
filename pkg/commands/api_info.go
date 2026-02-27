package commands

import (
	"context"
	"os"

	"github.com/spf13/cobra"
)

func addAPIInfo(topLevel *cobra.Command, opts *apiOptions) {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show resolved runtime configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAPI(cmd, opts, "info", func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				collections := runtime.Persistence.Collections(ctx, "")
				return map[string]any{
					"journal_source":   runtime.JournalSource,
					"collections":      collections,
					"collection_count": len(collections),
					"env": map[string]string{
						"BUJO_PATH":        os.Getenv("BUJO_PATH"),
						"BUJO_CONFIG_PATH": os.Getenv("BUJO_CONFIG_PATH"),
					},
				}, nil
			})
		},
	}

	topLevel.AddCommand(cmd)
}
