package entry

import (
	"fmt"
	"time"

	"tableflip.dev/bujo/pkg/glyph"
)

const (
	CurrentSchema = "v1"
)

func New(collection string, bullet glyph.Bullet, message string) *Entry {
	e := &Entry{
		Schema:     CurrentSchema,
		Created:    Timestamp{Time: time.Now()},
		Collection: collection,
		Signifier:  glyph.None,
		Bullet:     bullet,
		Message:    message,
	}
	e.ensureHistoryInitialized()
	e.appendHistory(HistoryActionAdded, "", collection, e.Created.Time)
	return e
}

type Entry struct {
	ID         string          `json:"-"` // do not json. ID is the filename.
	Bullet     glyph.Bullet    `json:"bullet"`
	Schema     string          `json:"schema"`
	Created    Timestamp       `json:"created"`
	Collection string          `json:"collection"`
	On         *Timestamp      `json:"on,omitempty"`
	Signifier  glyph.Signifier `json:"signifier,omitempty"`
	Message    string          `json:"message,omitempty"`
	History    []HistoryRecord `json:"history,omitempty"`
}

type HistoryAction string

const (
	HistoryActionAdded     HistoryAction = "added"
	HistoryActionMoved     HistoryAction = "moved"
	HistoryActionCompleted HistoryAction = "completed"
	HistoryActionStruck    HistoryAction = "struck"
)

type HistoryRecord struct {
	Timestamp Timestamp     `json:"timestamp"`
	Action    HistoryAction `json:"action"`
	From      string        `json:"from,omitempty"`
	To        string        `json:"to,omitempty"`
}

func (e *Entry) ensureHistoryInitialized() {
	if e.History == nil {
		e.History = make([]HistoryRecord, 0, 4)
	}
}

func (e *Entry) appendHistory(action HistoryAction, from, to string, at time.Time) {
	if e == nil {
		return
	}
	e.ensureHistoryInitialized()
	if at.IsZero() {
		at = time.Now()
	}
	e.History = append(e.History, HistoryRecord{
		Timestamp: Timestamp{Time: at},
		Action:    action,
		From:      from,
		To:        to,
	})
}

// EnsureHistorySeed populates a default history record for legacy entries that
// predate the history field.
func (e *Entry) EnsureHistorySeed() {
	if e == nil {
		return
	}
	if len(e.History) > 0 {
		return
	}
	e.ensureHistoryInitialized()
	created := e.Created.Time
	if created.IsZero() {
		created = time.Now()
	}
	e.History = append(e.History, HistoryRecord{
		Timestamp: Timestamp{Time: created},
		Action:    HistoryActionAdded,
		To:        e.Collection,
	})
	if e.Schema == "" {
		e.Schema = CurrentSchema
	}
}

func (e *Entry) Complete() {
	e.Bullet = glyph.Completed
	e.appendHistory(HistoryActionCompleted, e.Collection, e.Collection, time.Now())
}

func (e *Entry) Strike() {
	e.Bullet = glyph.Irrelevant
	e.Signifier = glyph.None
	e.appendHistory(HistoryActionStruck, e.Collection, e.Collection, time.Now())
}

func (e *Entry) Move(bullet glyph.Bullet, collection string) *Entry {
	e.ensureHistoryInitialized()
	ne := &Entry{
		ID:         "", // generate new id.
		Schema:     CurrentSchema,
		Created:    e.Created,
		Collection: collection,
		Signifier:  e.Signifier,
		Bullet:     e.Bullet,
		Message:    e.Message,
		History:    append([]HistoryRecord(nil), e.History...),
	}
	ne.ensureHistoryInitialized()
	e.Bullet = bullet
	now := time.Now()
	original := e.Collection
	e.appendHistory(HistoryActionMoved, original, collection, now)
	ne.appendHistory(HistoryActionMoved, original, collection, now)
	return ne
}

func (e *Entry) Title() string {
	return e.Collection
}

func (e *Entry) String() string {
	switch e.Bullet {
	case glyph.Completed:
		return fmt.Sprintf("%s %s  %s", glyph.None.String(), e.Bullet.String(), e.Message)
	default:
		return fmt.Sprintf("%s %s  %s", e.Signifier.String(), e.Bullet.String(), e.Message)
	}
}
