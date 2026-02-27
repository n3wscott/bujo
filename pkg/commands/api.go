package commands

import "github.com/spf13/cobra"

func addAPI(topLevel *cobra.Command) {
	opts := &apiOptions{}

	cmd := &cobra.Command{
		Use:           "api",
		Short:         "Machine-oriented API commands",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
		},
	}

	cmd.PersistentFlags().StringVar(&opts.Output, "output", apiOutputJSON, "Output format: json or text")
	cmd.PersistentFlags().StringVar(&opts.Journal, "journal", "", "Journal path override")

	addAPIInfo(cmd, opts)
	addAPICollections(cmd, opts)
	addAPIEntries(cmd, opts)
	addAPIReport(cmd, opts)

	topLevel.AddCommand(cmd)
}
