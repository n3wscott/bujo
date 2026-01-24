package addtask

import "github.com/charmbracelet/bubbles/v2/key"

// keyMap captures the interactive bindings for the add-task overlay.
type keyMap struct {
	NextField  key.Binding
	PrevField  key.Binding
	MoveUp     key.Binding
	MoveDown   key.Binding
	MoveLeft   key.Binding
	MoveRight  key.Binding
	Submit     key.Binding
	Cancel     key.Binding
	ConfirmYes key.Binding
	ConfirmNo  key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		NextField: key.NewBinding(key.WithKeys("tab")),
		PrevField: key.NewBinding(key.WithKeys("shift+tab")),
		MoveUp:    key.NewBinding(key.WithKeys("up", "k")),
		MoveDown:  key.NewBinding(key.WithKeys("down", "j")),
		MoveLeft:  key.NewBinding(key.WithKeys("left", "h")),
		MoveRight: key.NewBinding(key.WithKeys("right", "l")),
		Submit:    key.NewBinding(key.WithKeys("enter")),
		Cancel:    key.NewBinding(key.WithKeys("esc")),
		ConfirmYes: key.NewBinding(
			key.WithKeys("y", "enter"),
		),
		ConfirmNo: key.NewBinding(
			key.WithKeys("n", "esc"),
		),
	}
}
