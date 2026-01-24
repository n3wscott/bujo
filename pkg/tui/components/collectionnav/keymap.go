package collectionnav

import "github.com/charmbracelet/bubbles/v2/key"

// keyMap captures navigation bindings for the collection list.
type keyMap struct {
	Quit      key.Binding
	Select    key.Binding
	Expand    key.Binding
	Collapse  key.Binding
	MoveUp    key.Binding
	MoveDown  key.Binding
	MoveLeft  key.Binding
	MoveRight key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Quit:      key.NewBinding(key.WithKeys("q")),
		Select:    key.NewBinding(key.WithKeys("enter", " ")),
		Expand:    key.NewBinding(key.WithKeys("right", "l", "]")),
		Collapse:  key.NewBinding(key.WithKeys("left", "h", "[")),
		MoveUp:    key.NewBinding(key.WithKeys("up", "k")),
		MoveDown:  key.NewBinding(key.WithKeys("down", "j")),
		MoveLeft:  key.NewBinding(key.WithKeys("left", "h")),
		MoveRight: key.NewBinding(key.WithKeys("right", "l")),
	}
}
