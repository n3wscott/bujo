package app

import (
	tea "github.com/charmbracelet/bubbletea/v2"

	journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"
)

type pageKind int

const (
	pageKindJournal pageKind = iota
)

type pageModel interface {
	tea.Model
	SetSize(width, height int)
	View() (string, *tea.Cursor)
}

// pageRouter tracks the active main view and routes updates to it.
type pageRouter struct {
	active  pageKind
	journal *journalcomponent.Model
}

func newPageRouter() *pageRouter {
	return &pageRouter{
		active: pageKindJournal,
	}
}

func (r *pageRouter) SetJournal(journal *journalcomponent.Model) {
	r.journal = journal
}

func (r *pageRouter) Journal() *journalcomponent.Model {
	return r.journal
}

func (r *pageRouter) Init() tea.Cmd { return nil }

func (r *pageRouter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch r.active {
	case pageKindJournal:
		if r.journal == nil {
			return r, nil
		}
		next, cmd := r.journal.Update(msg)
		if jm, ok := next.(*journalcomponent.Model); ok {
			r.journal = jm
		}
		return r, cmd
	default:
		return r, nil
	}
}

func (r *pageRouter) ActiveView(width, height int) (string, *tea.Cursor, bool) {
	switch r.active {
	case pageKindJournal:
		if r.journal == nil {
			return "", nil, false
		}
		if height < 1 {
			height = 1
		}
		r.journal.SetSize(width, height)
		view, cursor := r.journal.View()
		return view, cursor, true
	default:
		return "", nil, false
	}
}
