package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/components/collectionnav"
	"tableflip.dev/bujo/pkg/tui/components/journal"
	"tableflip.dev/bujo/pkg/tui/events"
)

func newJournalCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "journal",
		Short: "Preview the combined journal view",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJournal(*opts)
		},
	}
	return cmd
}

func runJournal(opts options) error {
	const navID = events.ComponentID("MainNav")
	metas, collections, err := loadCollectionsData(opts.real)
	if err != nil {
		return err
	}
	nav := collectionnav.NewModel(collections)
	nav.SetID(navID)
	nav.SetFolded("Future", false)
	nav.SetFolded("Projects", true)
	nav.SetFolded("November 2025", true)

	sections, held, err := loadDetailSectionsData(opts.real, metas, collections, opts.hold)
	if err != nil {
		return err
	}
	detail := collectiondetail.NewModel(sections)
	detail.SetID(events.ComponentID("DetailPane"))
	detail.SetSourceNav(navID)

	cache := cachepkg.New(testbedFeedComponent)
	cache.SetCollections(metas)
	cache.SetSections(sections)
	registerHeldTemplates(cache, held)

	journalModel := journal.NewModel(nav, detail, cache)

	model := &journalTestModel{
		testbedModel: newTestbedModel(opts),
		journal:      journalModel,
		navID:        navID,
		cache:        cache,
		feeder:       newBulletFeeder(cache, held),
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

type journalTestModel struct {
	testbedModel
	journal *journal.Model
	navID   events.ComponentID
	cache   *cachepkg.Cache
	feeder  bulletFeeder
}

func (m *journalTestModel) Init() tea.Cmd {
	if m.journal == nil {
		return cacheListenCmd(m.cache)
	}
	return tea.Batch(m.journal.FocusNav(), cacheListenCmd(m.cache))
}

func (m *journalTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		if m.journal != nil {
			m.journal.SetSize(width, height)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case ".":
			m.feeder.Next()
		case "left", "h":
			if cmd := m.journal.FocusNav(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case events.CollectionSelectMsg:
		if msg.Component == m.navID {
			if cmd := m.journal.FocusDetail(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	if m.journal != nil {
		if _, cmd := m.journal.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *journalTestModel) View() (string, *tea.Cursor) {
	if m.journal == nil {
		return m.composeView("journal not configured", nil)
	}
	view, cursor := m.journal.View()
	return m.composeView(view, cursor)
}
