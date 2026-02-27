package app

import (
	"context"
	"time"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
)

// JournalService captures the service calls used by the TUI.
type JournalService interface {
	cachepkg.Service

	AddLabels(ctx context.Context, id string, labels []string) (*entry.Entry, error)
	Complete(ctx context.Context, id string) (*entry.Entry, error)
	EnsureCollectionOfType(ctx context.Context, name string, typ collection.Type) error
	Lock(ctx context.Context, id string) (*entry.Entry, error)
	Move(ctx context.Context, id, target string) (*entry.Entry, error)
	MigrationCandidates(ctx context.Context, since, until time.Time) ([]app.MigrationCandidate, error)
	Report(ctx context.Context, since, until time.Time) (app.ReportResult, error)
	RemoveLabels(ctx context.Context, id string, labels []string) (*entry.Entry, error)
	SetSignifier(ctx context.Context, id string, signifier glyph.Signifier) (*entry.Entry, error)
	Strike(ctx context.Context, id string) (*entry.Entry, error)
	ToggleSignifier(ctx context.Context, id string, signifier glyph.Signifier) (*entry.Entry, error)
	Unlock(ctx context.Context, id string) (*entry.Entry, error)
	Watch(ctx context.Context) (<-chan store.Event, error)
}
