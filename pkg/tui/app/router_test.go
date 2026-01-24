package app

import (
	"testing"

	journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"
)

func TestPageRouterActiveView(t *testing.T) {
	router := newPageRouter()
	if _, _, ok := router.ActiveView(40, 10); ok {
		t.Fatalf("expected no active view when journal is unset")
	}

	journal := journalcomponent.NewModel(nil, nil, nil)
	router.SetJournal(journal)
	view, _, ok := router.ActiveView(40, 10)
	if !ok {
		t.Fatalf("expected active view when journal is set")
	}
	if view == "" {
		t.Fatalf("expected non-empty view")
	}
}
