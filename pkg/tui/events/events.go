package events

import (
	"fmt"
	"time"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
)

// ComponentID uniquely identifies a component instance emitting events.
type ComponentID string

// CollectionRef captures the metadata required to identify a collection in
// cross-component events.
type CollectionRef struct {
	ID       string
	Name     string
	Type     collection.Type
	ParentID string
	Month    time.Time
	Day      time.Time
}

// Label returns a human-friendly identifier for the collection.
func (r CollectionRef) Label() string {
	if r.Name != "" {
		return r.Name
	}
	return r.ID
}

// CollectionHighlightMsg is emitted when a collection is highlighted (focused)
// within navigation components.
type CollectionHighlightMsg struct {
	Component  ComponentID
	Collection CollectionRef
	RowKind    string
}

// Describe renders the highlight in a human-friendly format for logs.
func (m CollectionHighlightMsg) Describe() string {
	return fmt.Sprintf(`name:%q type:%q`, m.Collection.Label(), m.RowKind)
}

// CollectionSelectMsg is emitted when the user activates a highlighted
// collection (e.g. presses Enter).
type CollectionSelectMsg struct {
	Component  ComponentID
	Collection CollectionRef
	RowKind    string
	Exists     bool
}

// Describe renders the selection in a human-friendly format for logs.
func (m CollectionSelectMsg) Describe() string {
	state := "missing"
	if m.Exists {
		state = "exists"
	}
	return fmt.Sprintf(`name:%q type:%q state:%q`, m.Collection.Label(), m.RowKind, state)
}

// RefFromParsed converts a ParsedCollection into an event reference.
func RefFromParsed(col *viewmodel.ParsedCollection) CollectionRef {
	if col == nil {
		return CollectionRef{}
	}
	return CollectionRef{
		ID:       col.ID,
		Name:     col.Name,
		Type:     col.Type,
		ParentID: col.ParentID,
		Month:    col.Month,
		Day:      col.Day,
	}
}
