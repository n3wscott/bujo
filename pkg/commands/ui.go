package commands

import (
    "tableflip.dev/bujo/pkg/store"

    "github.com/spf13/cobra"

    appsvc "tableflip.dev/bujo/pkg/app"
    teaui "tableflip.dev/bujo/pkg/runner/tea"
)

func addUI(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "open the text-based user interface",
		Example: `
bujo ui
`,
		ValidArgs: []string{},
        RunE: func(cmd *cobra.Command, args []string) error {
            // Load persistence
            p, err := store.Load(nil)
            if err != nil {
                return err
            }
            // Create shared service
            svc := &appsvc.Service{Persistence: p}
            // Run Bubble Tea UI
            return teaui.Run(svc)
        },
    }

	topLevel.AddCommand(cmd)
}
