package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/server"

	"tableflip.dev/bujo/pkg/store"
)

// Runner boots the MCP server over stdio.
type Runner struct {
	Persistence store.Persistence
	Name        string
	Version     string
}

// Run starts the Model Context Protocol server using stdio transport.
func Run(ctx context.Context, persistence store.Persistence) error {
	r := Runner{
		Persistence: persistence,
		Name:        "bujo",
		Version:     "dev",
	}
	return r.Do(ctx)
}

// Do executes the runner.
func (r Runner) Do(ctx context.Context) error {
	if r.Persistence == nil {
		return errors.New("mcp runner requires persistence")
	}
	name := r.Name
	if name == "" {
		name = "bujo"
	}
	version := r.Version
	if version == "" {
		version = "dev"
	}

	srv := server.NewMCPServer(
		fmt.Sprintf("%s MCP", name),
		version,
		server.WithResourceCapabilities(false, false),
		server.WithToolCapabilities(false),
		server.WithInstructions("Access bullet journal collections, entries, and mutations via MCP."),
		server.WithResourceRecovery(),
		server.WithRecovery(),
	)

	svc := NewService(r.Persistence)
	registerResources(srv, svc)
	registerTools(srv, svc)

	return server.ServeStdio(srv)
}
