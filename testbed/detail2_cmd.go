package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	collectiondetail2 "tableflip.dev/bujo/pkg/tui/components/collectiondetail2"
	"tableflip.dev/bujo/pkg/tui/events"
)

func newDetail2Cmd(opts *options) *cobra.Command {
	var mode string
	cmd := &cobra.Command{
		Use:   "detail2",
		Short: "Preview the collection detail pane (sectioned model)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetail2(*opts, mode)
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "continuous", "render mode: continuous or focused")
	return cmd
}

func runDetail2(opts options, mode string) error {
	metas, collections, err := loadCollectionsData(opts.real)
	if err != nil {
		return err
	}
	sections, held, err := loadDetailSectionsData(opts.real, metas, collections, opts.hold)
	if err != nil {
		return err
	}
	parsedMode, err := parseDetail2Mode(mode)
	if err != nil {
		return err
	}
	detail := collectiondetail2.NewModel(sections)
	detail.SetID(events.ComponentID("DetailPane2"))
	detail.SetMode(parsedMode)
	cache := cachepkg.New(testbedFeedComponent)
	cache.SetCollections(metas)
	cache.SetSections(sections)
	registerHeldTemplates(cache, held)
	model := &detail2TestModel{
		testbedModel: newTestbedModel(opts),
		detail:       detail,
		cache:        cache,
		feeder:       newBulletFeeder(cache, held),
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

type detail2TestModel struct {
	testbedModel
	detail *collectiondetail2.Model
	cache  *cachepkg.Cache
	feeder bulletFeeder
}

func (m *detail2TestModel) Init() tea.Cmd {
	return cacheListenCmd(m.cache)
}

func (m *detail2TestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	if _, cmd := m.testbedModel.Update(msg); cmd != nil { //nolint:staticcheck // invoke embedded base update
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
		case "m":
			next := collectiondetail2.ModeFocused
			if m.detail.Mode() == collectiondetail2.ModeFocused {
				next = collectiondetail2.ModeContinuous
			}
			m.detail.SetMode(next)
		}
		if isDetailNavKey(msg.String()) {
			if cmd := m.detail.Focus(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			m.SetFocus(true)
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

func (m *detail2TestModel) View() (string, *tea.Cursor) {
	return m.composeView(m.detail.View(), nil)
}
