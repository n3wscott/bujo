package commands

import (
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"os/exec"
)

func addUpgrade(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade bujo cli.",
		Example: `
bujo upgrade
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			ex := exec.Command("go", "install", "tableflip.dev/bujo/cmd/bujo@latest")
			var out bytes.Buffer
			ex.Stdout = &out
			if err := ex.Run(); err != nil {
				return output.HandleError(err)
			}
			fmt.Printf("%s\n", ex.String())
			return nil
		},
	}

	topLevel.AddCommand(cmd)
}
