package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/ui"
	"tableflip.dev/bujo/pkg/ui/common"
)

var (
	styles = common.DefaultStyles()

	rootCmd = &cobra.Command{
		Use:                   "charm",
		Short:                 "Do Charm stuff",
		Long:                  styles.Paragraph.Render(fmt.Sprintf("Do %s stuff. Run without arguments for a TUI or use the sub-commands like a pro.", styles.Keyword.Render("Charm"))),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if common.IsTTY() {
				return ui.NewProgram(nil).Start()
			}

			return cmd.Help()
		},
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
