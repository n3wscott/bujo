package events

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/glyph"
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

// ChangeType enumerates supported change actions across components.
type ChangeType string

const (
	// ChangeCreate indicates a new resource was created.
	ChangeCreate ChangeType = "create"
	// ChangeUpdate indicates an existing resource changed.
	ChangeUpdate ChangeType = "update"
	// ChangeDelete indicates a resource was removed.
	ChangeDelete ChangeType = "delete"
)

// CollectionChangeMsg announces structural updates to collections (create,
// rename, delete, re-type) regardless of their origin (user action, watcher,
// import, etc).
type CollectionChangeMsg struct {
	Component ComponentID
	Action    ChangeType
	Current   CollectionRef
	Previous  *CollectionRef
	Meta      map[string]string
}

// Describe implements the logging helper.
func (m CollectionChangeMsg) Describe() string {
	prev := ""
	if m.Previous != nil {
		prev = m.Previous.Label()
	}
	return fmt.Sprintf(`action:%q name:%q prev:%q type:%q`, m.Action, m.Current.Label(), prev, m.Current.Type)
}

// CollectionChangeCmd wraps CollectionChangeMsg into a tea.Cmd for callers that
// want to emit the event as part of an Update result.
func CollectionChangeCmd(component ComponentID, action ChangeType, current CollectionRef, prev *CollectionRef, meta map[string]string) tea.Cmd {
	return func() tea.Msg {
		return CollectionChangeMsg{
			Component: component,
			Action:    action,
			Current:   current,
			Previous:  prev,
			Meta:      meta,
		}
	}
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

// SectionRef describes a detail section emitting bullet events.
type CollectionViewRef struct {
	ID       string
	Title    string
	Subtitle string
}

// BulletRef describes a bullet/entry row within a collection.
type BulletRef struct {
	ID        string
	Label     string
	Note      string
	Bullet    glyph.Bullet
	Signifier glyph.Signifier
}

// BulletHighlightMsg fires whenever the detail pane highlights a bullet.
type BulletHighlightMsg struct {
	Component  ComponentID
	Collection CollectionViewRef
	Bullet     BulletRef
}

// Describe renders the bullet highlight for logs.
func (m BulletHighlightMsg) Describe() string {
	return fmt.Sprintf(`collection:%q bullet:%q`, m.Collection.Title, m.Bullet.Label)
}

// BulletSelectMsg fires when the user activates a bullet.
type BulletSelectMsg struct {
	Component  ComponentID
	Collection CollectionViewRef
	Bullet     BulletRef
	Exists     bool
}

// Describe renders the bullet selection for logs.
func (m BulletSelectMsg) Describe() string {
	state := "missing"
	if m.Exists {
		state = "exists"
	}
	return fmt.Sprintf(`collection:%q bullet:%q state:%q`, m.Collection.Title, m.Bullet.Label, state)
}

// BulletChangeMsg announces lifecycle changes to bullets (create/update/delete)
// so other components can refresh their state.
type BulletChangeMsg struct {
	Component  ComponentID
	Action     ChangeType
	Collection CollectionViewRef
	Bullet     BulletRef
	Meta       map[string]string
}

// Describe renders the change in a human-friendly format for logs.
func (m BulletChangeMsg) Describe() string {
	return fmt.Sprintf(`action:%q collection:%q bullet:%q`, m.Action, m.Collection.Title, m.Bullet.Label)
}

// BulletChangeCmd wraps BulletChangeMsg in a tea.Cmd.
func BulletChangeCmd(component ComponentID, action ChangeType, collection CollectionViewRef, bullet BulletRef, meta map[string]string) tea.Cmd {
	return func() tea.Msg {
		return BulletChangeMsg{
			Component:  component,
			Action:     action,
			Collection: collection,
			Bullet:     bullet,
			Meta:       meta,
		}
	}
}

// FocusMsg indicates a component just gained focus.
type FocusMsg struct {
	Component ComponentID
}

// Describe implements the logging helper.
func (m FocusMsg) Describe() string {
	return fmt.Sprintf(`component:%q state:"focus"`, m.Component)
}

// BlurMsg indicates a component just lost focus.
type BlurMsg struct {
	Component ComponentID
}

// Describe implements the logging helper.
func (m BlurMsg) Describe() string {
	return fmt.Sprintf(`component:%q state:"blur"`, m.Component)
}

// FocusCmd wraps a FocusMsg in a tea.Cmd helper.
func FocusCmd(component ComponentID) tea.Cmd {
	return func() tea.Msg {
		return FocusMsg{Component: component}
	}
}

// BlurCmd wraps a BlurMsg in a tea.Cmd helper.
func BlurCmd(component ComponentID) tea.Cmd {
	return func() tea.Msg {
		return BlurMsg{Component: component}
	}
}
