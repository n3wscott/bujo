package commands

import (
	"context"

	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/runner/mcp"
	"tableflip.dev/bujo/pkg/store"
)

func addMCP(topLevel *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "start the Model Context Protocol server",
		Long: `Launch an MCP server that exposes collections, entries, and journal commands
through the OpenAI-compatible Model Context Protocol.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			persistence, err := store.Load(nil)
			if err != nil {
				return err
			}
			return mcp.Run(context.Background(), persistence)
		},
	}

	topLevel.AddCommand(cmd)
}
