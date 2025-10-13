package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

func registerTools(srv *server.MCPServer, svc *Service) {
	registerCreateEntryTool(srv, svc)
	registerCompleteEntryTool(srv, svc)
	registerStrikeEntryTool(srv, svc)
	registerMoveEntryTool(srv, svc)
	registerUpdateBulletTool(srv, svc)
	registerUpdateSignifierTool(srv, svc)
	registerUpdateMessageTool(srv, svc)
	registerListEntriesTool(srv, svc)
	registerListCollectionsTool(srv, svc)
	registerSearchEntriesTool(srv, svc)
	registerGetEntryTool(srv, svc)
}

func registerCreateEntryTool(srv *server.MCPServer, svc *Service) {
	tool := mcp.NewTool(
		"create_entry",
		mcp.WithDescription("Create a new entry in a collection."),
		mcp.WithString("collection",
			mcp.Required(),
			mcp.Description("Collection that should hold the new entry."),
		),
		mcp.WithString("message",
			mcp.Description("Message or note to store with the entry."),
		),
		mcp.WithString("bullet",
			mcp.Description("Bullet type such as task, note, event, occurrence."),
			mcp.Enum("task", "note", "event", "occurrence", "comp", "movc", "movf", "irev"),
		),
		mcp.WithString("signifier",
			mcp.Description("Optional signifier such as priority (!), inspiration (!), investigation (?) or none."),
			mcp.Enum("none", "priority", "inspiration", "investigation"),
		),
		mcp.WithString("scheduled",
			mcp.Description("Optional RFC3339 timestamp representing the scheduled date."),
		),
	)

	srv.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Collection string `json:"collection"`
			Message    string `json:"message"`
			Bullet     string `json:"bullet"`
			Signifier  string `json:"signifier"`
			Scheduled  string `json:"scheduled"`
		}

		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid arguments: %v", err)), nil
		}

		bullet, err := ParseBullet(args.Bullet, glyph.Task)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		signifier, err := ParseSignifier(args.Signifier, glyph.None)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var scheduled *time.Time
		if strings.TrimSpace(args.Scheduled) != "" {
			when, err := entry.ParseTime(args.Scheduled)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid scheduled value: %v", err)), nil
			}
			scheduled = &when
		}

		dto, err := svc.AddEntry(ctx, AddEntryOptions{
			Collection: args.Collection,
			Message:    args.Message,
			Bullet:     bullet,
			Signifier:  signifier,
			On:         scheduled,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return toJSONResult(dto)
	})
}

func registerCompleteEntryTool(srv *server.MCPServer, svc *Service) {
	tool := mcp.NewTool(
		"complete_entry",
		mcp.WithDescription("Mark an entry as completed."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Entry identifier to complete."),
		),
	)

	srv.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := request.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dto, err := svc.CompleteEntry(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return toJSONResult(dto)
	})
}

func registerStrikeEntryTool(srv *server.MCPServer, svc *Service) {
	tool := mcp.NewTool(
		"strike_entry",
		mcp.WithDescription("Strike an entry to mark it irrelevant."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Entry identifier to strike."),
		),
	)

	srv.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := request.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dto, err := svc.StrikeEntry(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return toJSONResult(dto)
	})
}

func registerMoveEntryTool(srv *server.MCPServer, svc *Service) {
	tool := mcp.NewTool(
		"move_entry",
		mcp.WithDescription("Move an entry to a different collection."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Entry identifier to move."),
		),
		mcp.WithString("target_collection",
			mcp.Required(),
			mcp.Description("Destination collection for the cloned entry."),
		),
		mcp.WithString("moved_bullet",
			mcp.Description("Bullet applied to the source entry after moving."),
			mcp.Enum("movc", "movf", "comp", "task"),
		),
	)

	srv.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := request.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		target := request.GetString("target_collection", "")
		if target == "" {
			return mcp.NewToolResultError("target_collection is required"), nil
		}
		bulletValue := request.GetString("moved_bullet", string(glyph.MovedCollection))
		bullet, err := ParseBullet(bulletValue, glyph.MovedCollection)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		source, clone, err := svc.MoveEntry(ctx, MoveEntryOptions{
			ID:                id,
			TargetCollection:  target,
			ReplacementBullet: bullet,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return toJSONResult(map[string]*EntryDTO{
			"source": source,
			"clone":  clone,
		})
	})
}

func registerUpdateBulletTool(srv *server.MCPServer, svc *Service) {
	tool := mcp.NewTool(
		"update_bullet",
		mcp.WithDescription("Update the bullet glyph for an entry."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Entry identifier to modify."),
		),
		mcp.WithString("bullet",
			mcp.Required(),
			mcp.Description("New bullet value."),
			mcp.Enum("task", "note", "event", "comp", "movc", "movf", "irev", "occurrence"),
		),
	)

	srv.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := request.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		bulletRaw, err := request.RequireString("bullet")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		bullet, err := ParseBullet(bulletRaw, glyph.Task)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dto, err := svc.UpdateBullet(ctx, id, bullet)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return toJSONResult(dto)
	})
}

func registerUpdateSignifierTool(srv *server.MCPServer, svc *Service) {
	tool := mcp.NewTool(
		"update_signifier",
		mcp.WithDescription("Update the signifier associated with an entry."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Entry identifier to modify."),
		),
		mcp.WithString("signifier",
			mcp.Required(),
			mcp.Description("New signifier value."),
			mcp.Enum("none", "priority", "inspiration", "investigation"),
		),
	)

	srv.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := request.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		signifierRaw, err := request.RequireString("signifier")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		signifier, err := ParseSignifier(signifierRaw, glyph.None)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dto, err := svc.UpdateSignifier(ctx, id, signifier)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return toJSONResult(dto)
	})
}

func registerUpdateMessageTool(srv *server.MCPServer, svc *Service) {
	tool := mcp.NewTool(
		"update_message",
		mcp.WithDescription("Replace the message body for an entry."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Entry identifier to modify."),
		),
		mcp.WithString("message",
			mcp.Required(),
			mcp.Description("New message text."),
		),
	)

	srv.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := request.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		message, err := request.RequireString("message")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dto, err := svc.UpdateMessage(ctx, id, message)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return toJSONResult(dto)
	})
}

func registerListEntriesTool(srv *server.MCPServer, svc *Service) {
	tool := mcp.NewTool(
		"list_entries",
		mcp.WithDescription("List entries for a collection or the entire journal."),
		mcp.WithString("collection",
			mcp.Description("Optional collection filter."),
		),
	)

	srv.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		collection := strings.TrimSpace(request.GetString("collection", ""))
		var (
			results []EntryDTO
			err     error
		)
		if collection == "" {
			results, err = svc.ListAllEntries(ctx)
		} else {
			results, err = svc.ListEntries(ctx, collection)
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return toJSONResult(map[string]any{
			"collection": collection,
			"entries":    results,
			"count":      len(results),
		})
	})
}

func registerListCollectionsTool(srv *server.MCPServer, svc *Service) {
	tool := mcp.NewTool(
		"list_collections",
		mcp.WithDescription("List all collections tracked in the journal."),
	)

	srv.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		summaries, err := svc.ListCollections(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return toJSONResult(map[string]any{
			"collections": summaries,
			"count":       len(summaries),
		})
	})
}

func registerSearchEntriesTool(srv *server.MCPServer, svc *Service) {
	tool := mcp.NewTool(
		"search_entries",
		mcp.WithDescription("Search entries by substring match across messages and collections."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Case-insensitive search text."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of entries to return (default 20)."),
			mcp.Min(1),
			mcp.Max(100),
		),
	)

	srv.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := request.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		limit := request.GetInt("limit", 20)

		results, err := svc.SearchEntries(ctx, query, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return toJSONResult(map[string]any{
			"query":   query,
			"limit":   limit,
			"results": results,
			"count":   len(results),
		})
	})
}

func registerGetEntryTool(srv *server.MCPServer, svc *Service) {
	tool := mcp.NewTool(
		"get_entry",
		mcp.WithDescription("Fetch a single entry by identifier."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Entry identifier to fetch."),
		),
	)

	srv.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := request.RequireString("id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dto, err := svc.EntryByID(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return toJSONResult(dto)
	})
}

func toJSONResult(data any) (*mcp.CallToolResult, error) {
	result, err := mcp.NewToolResultJSON(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return result, nil
}
