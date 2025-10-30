package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
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
	metas, collections, err := loadCollectionsData(opts.real)
	if err != nil {
		return err
	}
	sections, held, err := loadDetailSectionsData(opts.real, metas, collections, opts.hold)
	if err != nil {
		return err
	}
	detail := collectiondetail.NewModel(sections)
	detail.SetID(events.ComponentID("DetailPane"))
	cache := cachepkg.New(testbedFeedComponent)
	cache.SetCollections(metas)
	cache.SetSections(sections)
	registerHeldTemplates(cache, held)
	model := &detailTestModel{
		testbedModel: newTestbedModel(opts),
		detail:       detail,
		cache:        cache,
		feeder:       newBulletFeeder(cache, held),
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

type detailTestModel struct {
	testbedModel
	detail *collectiondetail.Model
	cache  *cachepkg.Cache
	feeder bulletFeeder
}

func (m *detailTestModel) Init() tea.Cmd {
	return cacheListenCmd(m.cache)
}

func (m *detailTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cacheWrap, ok := msg.(cacheMsg); ok {
		cmds := []tea.Cmd{}
		if cmd := cacheListenCmd(m.cache); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if cacheWrap.payload == nil {
			return m, tea.Batch(cmds...)
		}
		model, innerCmd := m.Update(cacheWrap.payload)
		if innerCmd != nil {
			cmds = append(cmds, innerCmd)
		}
		return model, tea.Batch(cmds...)
	}
	var cmds []tea.Cmd
	if _, cmd := m.testbedModel.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width, height := m.contentSize()
		m.detail.SetSize(width, height)
	case tea.KeyMsg:
		switch msg.String() {
		case ".":
			m.feeder.Next()
		}
		if isDetailNavKey(msg.String()) {
			if cmd := m.detail.Focus(); cmd != nil {
				cmds = append(cmds, cmd)
			}
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

func (m *detailTestModel) View() (string, *tea.Cursor) {
	return m.composeView(m.detail.View(), nil)
}

func isDetailNavKey(key string) bool {
	switch key {
	case "up", "down", "k", "j", "pgup", "pgdown", "b", "f", "home", "end", "g", "G":
		return true
	default:
		return false
	}
}
