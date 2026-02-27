package commands

import (
	"time"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/entry"
)

type apiEntryPayload struct {
	ID         string              `json:"id"`
	Bullet     string              `json:"bullet"`
	Schema     string              `json:"schema,omitempty"`
	Collection string              `json:"collection"`
	Message    string              `json:"message,omitempty"`
	ParentID   string              `json:"parent_id,omitempty"`
	Immutable  bool                `json:"immutable"`
	Created    string              `json:"created,omitempty"`
	On         string              `json:"on,omitempty"`
	Signifier  string              `json:"signifier,omitempty"`
	History    []apiHistoryPayload `json:"history,omitempty"`
}

type apiHistoryPayload struct {
	Timestamp string `json:"timestamp,omitempty"`
	Action    string `json:"action"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
}

type apiReportItemPayload struct {
	Entry       apiEntryPayload `json:"entry"`
	Completed   bool            `json:"completed"`
	CompletedAt string          `json:"completed_at,omitempty"`
}

type apiReportSectionPayload struct {
	Collection string                 `json:"collection"`
	Entries    []apiReportItemPayload `json:"entries"`
}

func toAPIEntryPayload(e *entry.Entry) apiEntryPayload {
	if e == nil {
		return apiEntryPayload{}
	}
	payload := apiEntryPayload{
		ID:         e.ID,
		Bullet:     string(e.Bullet),
		Schema:     e.Schema,
		Collection: e.Collection,
		Message:    e.Message,
		ParentID:   e.ParentID,
		Immutable:  e.Immutable,
		Signifier:  string(e.Signifier),
		History:    make([]apiHistoryPayload, 0, len(e.History)),
	}
	if !e.Created.IsZero() {
		payload.Created = e.Created.UTC().Format(time.RFC3339)
	}
	if e.On != nil && !e.On.IsZero() {
		payload.On = e.On.UTC().Format(time.RFC3339)
	}
	for _, record := range e.History {
		item := apiHistoryPayload{
			Action: string(record.Action),
			From:   record.From,
			To:     record.To,
		}
		if !record.Timestamp.IsZero() {
			item.Timestamp = record.Timestamp.UTC().Format(time.RFC3339)
		}
		payload.History = append(payload.History, item)
	}
	return payload
}

func toAPIEntryPayloads(entries []*entry.Entry) []apiEntryPayload {
	payloads := make([]apiEntryPayload, 0, len(entries))
	for _, e := range entries {
		payloads = append(payloads, toAPIEntryPayload(e))
	}
	return payloads
}

func toAPIReportSections(sections []app.ReportSection) []apiReportSectionPayload {
	out := make([]apiReportSectionPayload, 0, len(sections))
	for _, section := range sections {
		items := make([]apiReportItemPayload, 0, len(section.Entries))
		for _, item := range section.Entries {
			payload := apiReportItemPayload{
				Entry:     toAPIEntryPayload(item.Entry),
				Completed: item.Completed,
			}
			if !item.CompletedAt.IsZero() {
				payload.CompletedAt = item.CompletedAt.UTC().Format(time.RFC3339)
			}
			items = append(items, payload)
		}
		out = append(out, apiReportSectionPayload{
			Collection: section.Collection,
			Entries:    items,
		})
	}
	return out
}
