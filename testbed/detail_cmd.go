package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

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
		m.detail.SetSize(width, height)
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
	return []collectiondetail.Section{
		{
			ID:    "inbox",
			Title: "Inbox",
			Bullets: []collectiondetail.Bullet{
				{ID: "1", Label: "Draft release notes", Note: "task"},
				{ID: "2", Label: "Review pull requests", Note: "2 pending"},
				{ID: "3", Label: "Email Alex about the demo"},
				{ID: "8", Label: "Write a really long description for this task so we can verify wrapping behaves correctly when the line length greatly exceeds the available width in the detail pane"},
			},
		},
		{
			ID:       "today",
			Title:    "Today",
			Subtitle: "Friday · October 24",
			Bullets: []collectiondetail.Bullet{
				{ID: "4", Label: "Standup", Note: "09:30"},
				{ID: "5", Label: "Ship calendar refactor"},
				{ID: "6", Label: "Plan weekend hike"},
			},
		},
		{
			ID:    "future",
			Title: "Future",
			Bullets: []collectiondetail.Bullet{
				{ID: "7", Label: "Book flights to NYC"},
			},
		},
	}
}
