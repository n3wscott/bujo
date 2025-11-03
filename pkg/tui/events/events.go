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

// CollectionViewRef describes a view-layer collection section emitting bullet events.
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

// BulletCompleteMsg requests that a bullet be marked completed.
type BulletCompleteMsg struct {
	Component  ComponentID
	Collection CollectionViewRef
	Bullet     BulletRef
}

func (m BulletCompleteMsg) Describe() string {
	return fmt.Sprintf(`collection:%q bullet:%q`, m.Collection.Title, m.Bullet.Label)
}

// BulletStrikeMsg requests that a bullet be marked irrelevant.
type BulletStrikeMsg struct {
	Component  ComponentID
	Collection CollectionViewRef
	Bullet     BulletRef
}

func (m BulletStrikeMsg) Describe() string {
	return fmt.Sprintf(`collection:%q bullet:%q`, m.Collection.Title, m.Bullet.Label)
}

// BulletMoveFutureMsg requests that a bullet be migrated to the Future log.
type BulletMoveFutureMsg struct {
	Component  ComponentID
	Collection CollectionViewRef
	Bullet     BulletRef
}

func (m BulletMoveFutureMsg) Describe() string {
	return fmt.Sprintf(`collection:%q bullet:%q target:%q`, m.Collection.Title, m.Bullet.Label, "Future")
}

// BulletSignifierMsg requests the signifier on a bullet be updated.
type BulletSignifierMsg struct {
	Component  ComponentID
	Collection CollectionViewRef
	Bullet     BulletRef
	Signifier  glyph.Signifier
}

func (m BulletSignifierMsg) Describe() string {
	return fmt.Sprintf(`collection:%q bullet:%q signifier:%q`, m.Collection.Title, m.Bullet.Label, m.Signifier)
}

// MoveBulletRequestMsg asks the app to move a bullet to a new collection.
type MoveBulletRequestMsg struct {
	Component  ComponentID
	Collection CollectionViewRef
	Bullet     BulletRef
}

// Describe renders the move request for logs.
func (m MoveBulletRequestMsg) Describe() string {
	return fmt.Sprintf(`component:%q collection:%q bullet:%q`, m.Component, m.Collection.Title, m.Bullet.Label)
}

// MoveBulletRequestCmd wraps MoveBulletRequestMsg in a tea.Cmd.
func MoveBulletRequestCmd(component ComponentID, collection CollectionViewRef, bullet BulletRef) tea.Cmd {
	return func() tea.Msg {
		return MoveBulletRequestMsg{
			Component:  component,
			Collection: collection,
			Bullet:     bullet,
		}
	}
}

// CollectionOrderMsg announces that the ordering of collections changed and
// carries the flattened ID list so listeners can reorder their local state.
type CollectionOrderMsg struct {
	Component ComponentID
	Order     []string
}

// Describe renders the order change for logs.
func (m CollectionOrderMsg) Describe() string {
	return fmt.Sprintf(`component:%q order:%d`, m.Component, len(m.Order))
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

// AddTaskRequestMsg asks the root model to open the add-task overlay for the
// provided collection context.
type AddTaskRequestMsg struct {
	Component         ComponentID
	CollectionID      string
	CollectionLabel   string
	ParentBulletID    string
	ParentBulletLabel string
	Origin            string
}

// Describe renders the request for logs.
func (m AddTaskRequestMsg) Describe() string {
	return fmt.Sprintf(`component:%q collection:%q parent:%q origin:%q`,
		m.Component, m.CollectionLabel, m.ParentBulletLabel, m.Origin)
}

// AddTaskRequestCmd wraps AddTaskRequestMsg in a tea.Cmd.
func AddTaskRequestCmd(component ComponentID, collectionID, collectionLabel, parentID, parentLabel, origin string) tea.Cmd {
	return func() tea.Msg {
		return AddTaskRequestMsg{
			Component:         component,
			CollectionID:      collectionID,
			CollectionLabel:   collectionLabel,
			ParentBulletID:    parentID,
			ParentBulletLabel: parentLabel,
			Origin:            origin,
		}
	}
}

// BulletDetailRequestMsg asks the root model to display metadata for a bullet.
type BulletDetailRequestMsg struct {
	Component  ComponentID
	Collection CollectionViewRef
	Bullet     BulletRef
}

// Describe renders the bullet detail request for logs.
func (m BulletDetailRequestMsg) Describe() string {
	return fmt.Sprintf(`component:%q collection:%q bullet:%q`,
		m.Component, m.Collection.Title, m.Bullet.Label)
}

// BulletDetailRequestCmd wraps BulletDetailRequestMsg in a tea.Cmd.
func BulletDetailRequestCmd(component ComponentID, collection CollectionViewRef, bullet BulletRef) tea.Cmd {
	return func() tea.Msg {
		return BulletDetailRequestMsg{
			Component:  component,
			Collection: collection,
			Bullet:     bullet,
		}
	}
}

// CommandMode represents the current state of the command prompt.
type CommandMode string

const (
	// CommandModePassive indicates the command bar is idle.
	CommandModePassive CommandMode = "passive"
	// CommandModeInput indicates the command bar is collecting user input.
	CommandModeInput CommandMode = "input"
)

// CommandChangeMsg is emitted when the command input value changes.
type CommandChangeMsg struct {
	Component ComponentID
	Value     string
	Mode      CommandMode
}

// Describe implements the logging helper.
func (m CommandChangeMsg) Describe() string {
	return fmt.Sprintf(`value:%q mode:%q`, m.Value, m.Mode)
}

// CommandSubmitMsg is emitted when the command input is submitted.
type CommandSubmitMsg struct {
	Component ComponentID
	Value     string
}

// Describe implements the logging helper.
func (m CommandSubmitMsg) Describe() string {
	return fmt.Sprintf(`value:%q`, m.Value)
}

// CommandCancelMsg is emitted when command entry is cancelled.
type CommandCancelMsg struct {
	Component ComponentID
}

// Describe implements the logging helper.
func (m CommandCancelMsg) Describe() string {
	return fmt.Sprintf(`component:%q`, m.Component)
}

// CommandChangeCmd wraps CommandChangeMsg.
func CommandChangeCmd(component ComponentID, value string, mode CommandMode) tea.Cmd {
	return func() tea.Msg {
		return CommandChangeMsg{
			Component: component,
			Value:     value,
			Mode:      mode,
		}
	}
}

// CommandSubmitCmd wraps CommandSubmitMsg.
func CommandSubmitCmd(component ComponentID, value string) tea.Cmd {
	return func() tea.Msg {
		return CommandSubmitMsg{
			Component: component,
			Value:     value,
		}
	}
}

// CommandCancelCmd wraps CommandCancelMsg.
func CommandCancelCmd(component ComponentID) tea.Cmd {
	return func() tea.Msg {
		return CommandCancelMsg{
			Component: component,
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

// DebugMsg captures optional diagnostic notes emitted by components.
type DebugMsg struct {
	Component ComponentID
	Context   string
	Detail    string
}

// Describe renders the debug message in a human‑readable format.
func (m DebugMsg) Describe() string {
	return fmt.Sprintf(`component:%q context:%q detail:%q`, m.Component, m.Context, m.Detail)
}

// DebugCmd wraps DebugMsg creation in a tea.Cmd helper.
func DebugCmd(component ComponentID, context, detail string) tea.Cmd {
	return func() tea.Msg {
		return DebugMsg{Component: component, Context: context, Detail: detail}
	}
}
