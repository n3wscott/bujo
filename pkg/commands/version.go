package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	goversion "go.hein.dev/go-version"
)

func addVersion(topLevel *cobra.Command) {
	shortened := false
	version := "dev"
	commit := "none"
	date := "unknown"
	output := "json"
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Get bujo version.",
		Example: `
bujo version
`,
		Run: func(_ *cobra.Command, _ []string) {
			resp := goversion.FuncWithOutput(shortened, version, commit, date, output)
			fmt.Print(resp)
		},
	}

	cmd.Flags().BoolVarP(&shortened, "short", "s", false, "Print just the version number.")
	cmd.Flags().StringVarP(&output, "output", "o", "json", "Output format. One of 'yaml' or 'json'.")

	topLevel.AddCommand(cmd)
}
