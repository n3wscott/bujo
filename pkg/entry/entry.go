// Package entry defines the core bullet journal entry model.
package entry

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"tableflip.dev/bujo/pkg/glyph"
)

// CurrentSchema identifies the persisted entry schema version.
const (
	CurrentSchema = "v1"
)

// New constructs a new entry for the provided collection.
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

// Entry represents a single bullet journal entry.
type Entry struct {
	ID         string          `json:"-"` // do not json. ID is the filename.
	Bullet     glyph.Bullet    `json:"bullet"`
	Schema     string          `json:"schema"`
	Created    Timestamp       `json:"created"`
	Collection string          `json:"collection"`
	On         *Timestamp      `json:"on,omitempty"`
	Signifier  glyph.Signifier `json:"signifier,omitempty"`
	Message    string          `json:"message,omitempty"`
	Labels     []string        `json:"labels,omitempty"`
	ParentID   string          `json:"parent_id,omitempty"`
	Immutable  bool            `json:"immutable,omitempty"`
	History    []HistoryRecord `json:"history,omitempty"`
}

// HistoryAction describes a recorded change in an entry's history.
type HistoryAction string

const (
	// HistoryActionAdded indicates the entry was created.
	HistoryActionAdded HistoryAction = "added"
	// HistoryActionMoved indicates the entry was migrated to another collection.
	HistoryActionMoved HistoryAction = "moved"
	// HistoryActionCompleted indicates the entry was completed.
	HistoryActionCompleted HistoryAction = "completed"
	// HistoryActionStruck indicates the entry was struck out as irrelevant.
	HistoryActionStruck HistoryAction = "struck"
)

// HistoryRecord captures a single history event for an entry.
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
	e.NormalizeLabels()
}

// LastCompletionTime reports the most recent timestamp the entry was completed.
func (e *Entry) LastCompletionTime() (time.Time, bool) {
	if e == nil {
		return time.Time{}, false
	}
	var latest time.Time
	found := false
	for _, record := range e.History {
		if record.Action != HistoryActionCompleted {
			continue
		}
		ts := record.Timestamp.Time
		if ts.IsZero() {
			continue
		}
		if !found || ts.After(latest) {
			latest = ts
			found = true
		}
	}
	return latest, found
}

// Complete marks the entry as completed.
func (e *Entry) Complete() {
	e.Bullet = glyph.Completed
	e.appendHistory(HistoryActionCompleted, e.Collection, e.Collection, time.Now())
}

// Strike marks the entry as irrelevant.
func (e *Entry) Strike() {
	e.Bullet = glyph.Irrelevant
	e.Signifier = glyph.None
	e.appendHistory(HistoryActionStruck, e.Collection, e.Collection, time.Now())
}

// Move clones the entry into a new collection and marks the original as moved.
func (e *Entry) Move(bullet glyph.Bullet, collection string) *Entry {
	e.ensureHistoryInitialized()
	e.NormalizeLabels()
	ne := &Entry{
		ID:         "", // generate new id.
		Schema:     CurrentSchema,
		Created:    e.Created,
		Collection: collection,
		Signifier:  e.Signifier,
		Bullet:     e.Bullet,
		Message:    e.Message,
		Labels:     append([]string(nil), e.Labels...),
		ParentID:   "",
		History:    append([]HistoryRecord(nil), e.History...),
	}
	ne.ensureHistoryInitialized()
	e.Bullet = bullet
	e.Immutable = true
	now := time.Now()
	original := e.Collection
	e.appendHistory(HistoryActionMoved, original, collection, now)
	ne.appendHistory(HistoryActionMoved, original, collection, now)
	return ne
}

// Lock marks the entry immutable.
func (e *Entry) Lock() {
	if e == nil {
		return
	}
	e.Immutable = true
}

// Unlock removes the immutable flag.
func (e *Entry) Unlock() {
	if e == nil {
		return
	}
	e.Immutable = false
}

// CloneToCollection creates a deep copy destined for the provided collection
// while preserving bullet, signifier, message, and parent linkage.
func (e *Entry) CloneToCollection(collection string) *Entry {
	e.NormalizeLabels()
	clone := &Entry{
		Schema:     CurrentSchema,
		Created:    e.Created,
		Collection: collection,
		Signifier:  e.Signifier,
		Bullet:     e.Bullet,
		Message:    e.Message,
		Labels:     append([]string(nil), e.Labels...),
		ParentID:   e.ParentID,
		History:    append([]HistoryRecord(nil), e.History...),
	}
	clone.ensureHistoryInitialized()
	return clone
}

// NormalizeLabels canonicalizes labels in place: trim/lowercase/dedupe/sort.
func (e *Entry) NormalizeLabels() {
	if e == nil {
		return
	}
	e.Labels = NormalizeLabels(e.Labels)
}

// SetLabels replaces labels with the canonicalized input.
func (e *Entry) SetLabels(labels []string) {
	if e == nil {
		return
	}
	e.Labels = NormalizeLabels(labels)
}

// AddLabels merges labels into the current canonicalized set.
func (e *Entry) AddLabels(labels []string) {
	if e == nil {
		return
	}
	merged := append(append([]string(nil), e.Labels...), labels...)
	e.Labels = NormalizeLabels(merged)
}

// RemoveLabels removes matching labels from the canonicalized set.
func (e *Entry) RemoveLabels(labels []string) {
	if e == nil {
		return
	}
	current := NormalizeLabels(e.Labels)
	remove := NormalizeLabels(labels)
	if len(remove) == 0 {
		e.Labels = current
		return
	}
	deny := make(map[string]struct{}, len(remove))
	for _, label := range remove {
		deny[label] = struct{}{}
	}
	out := make([]string, 0, len(current))
	for _, label := range current {
		if _, blocked := deny[label]; blocked {
			continue
		}
		out = append(out, label)
	}
	e.Labels = out
}

// NormalizeLabels returns canonical labels: trim/lowercase/dedupe/sort.
func NormalizeLabels(labels []string) []string {
	if len(labels) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(labels))
	out := make([]string, 0, len(labels))
	for _, raw := range labels {
		label := strings.ToLower(strings.TrimSpace(raw))
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		out = append(out, label)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// Title returns the entry's collection name for presentation.
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
