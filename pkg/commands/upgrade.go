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
		RunE: func(cmd *cobra.Command, args []string) error {
			ex := exec.Command("go", "get", "-u", "tableflip.dev/bujo/cmd/bujo")
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
