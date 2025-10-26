package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
)

func newDetailCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detail",
		Short: "Preview the collection detail pane",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetail(*opts)
		},
	}
	return cmd
}

func runDetail(opts options) error {
	detail := collectiondetail.NewModel(sampleDetailSections())
	model := &detailTestModel{
		testbedModel: testbedModel{
			fullscreen: opts.full,
			maxWidth:   opts.width,
			maxHeight:  opts.height,
		},
		detail: detail,
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type detailTestModel struct {
	testbedModel
	detail *collectiondetail.Model
}

func (m *detailTestModel) Init() tea.Cmd { return nil }

func (m *detailTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if _, cmd := m.testbedModel.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width, height := m.contentSize()
		m.detail.SetSize(width-2, height-2)
	case tea.KeyMsg:
		if isDetailNavKey(msg.String()) {
			m.detail.Focus()
			m.testbedModel.SetFocus(true)
		}
	}
	if _, cmd := m.detail.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *detailTestModel) View() string {
	content := m.detail.View()
	frame := m.renderFrame(content)
	if m.fullscreen {
		return frame
	}
	return m.placeFrame(frame)
}

func isDetailNavKey(key string) bool {
	switch key {
	case "up", "down", "k", "j", "pgup", "pgdown", "b", "f", "home", "end", "g", "G":
		return true
	default:
		return false
	}
}

func sampleDetailSections() []collectiondetail.Section {
	now := time.Now()
	return []collectiondetail.Section{
		{
			ID:    "inbox",
			Title: "Inbox",
			Bullets: []collectiondetail.Bullet{
				{
					ID:        "1",
					Label:     "Draft release notes",
					Bullet:    glyph.Task,
					Signifier: glyph.Priority,
					Created:   now.Add(-2 * time.Hour),
				},
				{
					ID:        "2",
					Label:     "Review pull requests",
					Bullet:    glyph.Event,
					Signifier: glyph.None,
					Created:   now.Add(-90 * time.Minute),
					Children: []collectiondetail.Bullet{
						{
							ID:        "2-1",
							Label:     "UI polish PR",
							Bullet:    glyph.Task,
							Signifier: glyph.Priority,
							Created:   now.Add(-80 * time.Minute),
						},
						{
							ID:      "2-2",
							Label:   "Write an extra long review comment about the storage refactor PR so we can verify wrapping works for nested subtasks that exceed the available width in the detail pane",
							Bullet:  glyph.Task,
							Created: now.Add(-70 * time.Minute),
						},
					},
				},
				{
					ID:        "3",
					Label:     "Email Alex about the demo",
					Bullet:    glyph.Note,
					Signifier: glyph.Inspiration,
					Created:   now.Add(-30 * time.Minute),
				},
				{
					ID:        "8",
					Label:     "Write a really long description for this task so we can verify wrapping behaves correctly when the line length greatly exceeds the available width in the detail pane",
					Bullet:    glyph.Task,
					Signifier: glyph.None,
					Created:   now,
				},
				{
					ID:        "9",
					Label:     "Archive old OKR doc",
					Bullet:    glyph.MovedCollection,
					Signifier: glyph.None,
					Created:   now.Add(-15 * time.Minute),
				},
			},
		},
		{
			ID:       "today",
			Title:    "Today",
			Subtitle: "Friday · October 24",
			Bullets: []collectiondetail.Bullet{
				{ID: "4", Label: "Standup", Bullet: glyph.Event, Created: now.Add(-4 * time.Hour)},
				{ID: "5", Label: "Ship calendar refactor", Bullet: glyph.Task, Signifier: glyph.Investigation, Created: now.Add(-3 * time.Hour)},
				{ID: "6", Label: "Plan weekend hike", Bullet: glyph.Note, Created: now.Add(-2 * time.Hour)},
				{
					ID:        "10",
					Label:     "Send launch email to list (done!)",
					Bullet:    glyph.Completed,
					Signifier: glyph.None,
					Created:   now.Add(-1 * time.Hour),
				},
				{
					ID:        "11",
					Label:     "Purge deprecated scripts (no longer needed)",
					Bullet:    glyph.Irrelevant,
					Signifier: glyph.None,
					Created:   now.Add(-30 * time.Minute),
				},
			},
		},
		{
			ID:    "future",
			Title: "Future",
			Bullets: []collectiondetail.Bullet{
				{ID: "7", Label: "Book flights to NYC", Bullet: glyph.Task, Created: now.Add(24 * time.Hour)},
				{ID: "12", Label: "Reschedule dentist appointment", Bullet: glyph.MovedFuture, Created: now.Add(48 * time.Hour)},
			},
		},
	}
}
