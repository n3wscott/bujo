package newapp

import (
	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/components/bulletdetail"
	"tableflip.dev/bujo/pkg/tui/components/command"
)

type bulletdetailOverlay struct {
	model *bulletdetail.Model
}

func newBulletdetailOverlay(model *bulletdetail.Model) *bulletdetailOverlay {
	return &bulletdetailOverlay{model: model}
}

func (o *bulletdetailOverlay) Init() tea.Cmd {
	if o.model == nil {
		return nil
	}
	return o.model.Init()
}

func (o *bulletdetailOverlay) Update(msg tea.Msg) (command.Overlay, tea.Cmd) {
	if o.model == nil {
		return o, nil
	}
	next, cmd := o.model.Update(msg)
	if m, ok := next.(*bulletdetail.Model); ok {
		o.model = m
	}
	return o, cmd
}

func (o *bulletdetailOverlay) View() (string, *tea.Cursor) {
	if o.model == nil {
		return "", nil
	}
	return o.model.View()
}

func (o *bulletdetailOverlay) SetSize(width, height int) {
	if o.model == nil {
		return
	}
	o.model.SetSize(width, height)
}

func (o *bulletdetailOverlay) Model() *bulletdetail.Model {
	return o.model
}
