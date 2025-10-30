package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	lipgloss "github.com/charmbracelet/lipgloss/v2"
	"github.com/spf13/cobra"

	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/components/addtask"
)

func newAddCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Preview the add-task overlay",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(*opts)
		},
	}
	return cmd
}

func runAdd(opts options) error {
	metas, collections, err := loadCollectionsData(opts.real)
	if err != nil {
		return err
	}
	sections, held, err := loadDetailSectionsData(opts.real, metas, collections, opts.hold)
	if err != nil {
		return err
	}

	cache := cachepkg.New(testbedFeedComponent)
	cache.SetCollections(metas)
	cache.SetSections(sections)
	registerHeldTemplates(cache, held)

	var initialCollection string
	if len(metas) > 0 {
		initialCollection = metas[0].Name
	}

	addView := addtask.NewModel(cache, addtask.Options{
		InitialCollectionID: initialCollection,
	})

	model := &addTestModel{
		testbedModel: newTestbedModel(opts),
		cache:        cache,
		add:          addView,
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

type addTestModel struct {
	testbedModel
	cache *cachepkg.Cache
	add   *addtask.Model
}

func (m *addTestModel) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	if m.cache != nil {
		cmds = append(cmds, cacheListenCmd(m.cache))
	}
	if m.add != nil {
		if initCmd := m.add.Init(); initCmd != nil {
			cmds = append(cmds, initCmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *addTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cacheWrap, ok := msg.(cacheMsg); ok {
		cmds := []tea.Cmd{}
		if cmd := cacheListenCmd(m.cache); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if cacheWrap.payload != nil && m.add != nil {
			if _, cmd := m.add.Update(cacheWrap.payload); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if len(cmds) == 0 {
			return m, nil
		}
		return m, tea.Batch(cmds...)
	}

	var cmds []tea.Cmd
	if _, cmd := m.testbedModel.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		if m.add != nil {
			width, height := m.contentSize()
			m.add.SetSize(width, height)
			if _, cmd := m.add.Update(v); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case tea.KeyMsg:
		if m.add != nil {
			if _, cmd := m.add.Update(v); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	default:
		if m.add != nil {
			if _, cmd := m.add.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *addTestModel) View() (string, *tea.Cursor) {
	if m.add == nil {
		return m.composeView("add task view unavailable", nil)
	}
	width, height := m.contentSize()
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	view, cursor := m.add.View()
	offsetX := 0
	if w := lipgloss.Width(view); w < width {
		offsetX = (width - w) / 2
	}
	content := lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Top,
		view,
		lipgloss.WithWhitespaceChars(" "),
	)
	if cursor != nil {
		cursor = offsetCursor(cursor, offsetX, 0)
	}
	return m.composeView(content, cursor)
}
