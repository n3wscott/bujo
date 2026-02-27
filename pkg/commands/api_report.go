package commands

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/timeutil"
)

func addAPIReport(topLevel *cobra.Command, opts *apiOptions) {
	var last string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Return completed-entry report data",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			duration, label, err := timeutil.ParseWindow(last)
			if err != nil {
				return apiFailure(cmd, opts, "report", expandUserPath(opts.Journal), newAPIError("invalid_argument", err.Error()))
			}
			until := time.Now()
			since := until.Add(-duration)

			return runAPI(cmd, opts, "report", func(ctx context.Context, runtime apiRuntime) (map[string]any, error) {
				result, err := runtime.Service.Report(ctx, since, until)
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"window": map[string]string{
						"label": label,
						"since": result.Since.UTC().Format(time.RFC3339),
						"until": result.Until.UTC().Format(time.RFC3339),
					},
					"total":    result.Total,
					"sections": toAPIReportSections(result.Sections),
				}, nil
			})
		},
	}

	cmd.Flags().StringVar(&last, "last", timeutil.DefaultWindow, "time window to include (for example 3d, 1w)")
	topLevel.AddCommand(cmd)
}
