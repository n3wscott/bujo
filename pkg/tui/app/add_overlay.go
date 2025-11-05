package app

import (
	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/components/addtask"
	"tableflip.dev/bujo/pkg/tui/components/command"
)

type addtaskOverlay struct {
	model *addtask.Model
}

func newAddtaskOverlay(model *addtask.Model) *addtaskOverlay {
	return &addtaskOverlay{model: model}
}

func (o *addtaskOverlay) Init() tea.Cmd {
	if o.model == nil {
		return nil
	}
	return o.model.Init()
}

func (o *addtaskOverlay) Update(msg tea.Msg) (command.Overlay, tea.Cmd) {
	if o.model == nil {
		return o, nil
	}
	next, cmd := o.model.Update(msg)
	if m, ok := next.(*addtask.Model); ok {
		o.model = m
	}
	return o, cmd
}

func (o *addtaskOverlay) View() (string, *tea.Cursor) {
	if o.model == nil {
		return "", nil
	}
	return o.model.View()
}

func (o *addtaskOverlay) SetSize(width, height int) {
	if o.model == nil {
		return
	}
	o.model.SetSize(width, height)
}

func (o *addtaskOverlay) Model() *addtask.Model {
	return o.model
}
