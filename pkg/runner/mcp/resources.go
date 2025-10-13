package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerResources(srv *server.MCPServer, svc *Service) {
	registerCollectionsResource(srv, svc)
	registerCollectionTemplate(srv, svc)
	registerEntryTemplate(srv, svc)
}

func registerCollectionsResource(srv *server.MCPServer, svc *Service) {
	resource := mcp.NewResource(
		"bujo://collections",
		"Collections",
		mcp.WithResourceDescription("All available bullet journal collections with counts."),
		mcp.WithMIMEType("application/json"),
	)

	srv.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		summaries, err := svc.ListCollections(ctx)
		if err != nil {
			return nil, err
		}

		payload := map[string]any{
			"collections": summaries,
			"count":       len(summaries),
		}
		return encodeResourceJSON(request.Params.URI, payload)
	})
}

func registerCollectionTemplate(srv *server.MCPServer, svc *Service) {
	template := mcp.NewResourceTemplate(
		"bujo://collections/{name}",
		"Collection Entries",
		mcp.WithTemplateDescription("Entries that belong to a collection."),
		mcp.WithTemplateMIMEType("application/json"),
	)

	srv.AddResourceTemplate(template, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		name, _ := request.Params.Arguments["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("collection name is required")
		}

		entries, err := svc.ListEntries(ctx, name)
		if err != nil {
			return nil, err
		}

		payload := map[string]any{
			"collection": name,
			"count":      len(entries),
			"entries":    entries,
		}
		return encodeResourceJSON(request.Params.URI, payload)
	})
}

func registerEntryTemplate(srv *server.MCPServer, svc *Service) {
	template := mcp.NewResourceTemplate(
		"bujo://entries/{id}",
		"Entry Details",
		mcp.WithTemplateDescription("Detailed information about a single entry."),
		mcp.WithTemplateMIMEType("application/json"),
	)

	srv.AddResourceTemplate(template, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		id, _ := request.Params.Arguments["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("entry id is required")
		}

		dto, err := svc.EntryByID(ctx, id)
		if err != nil {
			return nil, err
		}

		payload := map[string]any{
			"entry": dto,
		}
		return encodeResourceJSON(request.Params.URI, payload)
	})
}

func encodeResourceJSON(uri string, payload any) ([]mcp.ResourceContents, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}
