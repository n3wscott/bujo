package command

import "github.com/charmbracelet/bubbles/v2/key"

// keyMap captures command prompt input bindings.
type keyMap struct {
	StartInput     key.Binding
	CancelInput    key.Binding
	Submit         key.Binding
	SuggestPrev    key.Binding
	SuggestNext    key.Binding
	ConfirmSuggest key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		StartInput:     key.NewBinding(key.WithKeys(":")),
		CancelInput:    key.NewBinding(key.WithKeys("esc")),
		Submit:         key.NewBinding(key.WithKeys("enter")),
		SuggestPrev:    key.NewBinding(key.WithKeys("up", "shift+tab")),
		SuggestNext:    key.NewBinding(key.WithKeys("down", "tab")),
		ConfirmSuggest: key.NewBinding(key.WithKeys("enter")),
	}
}
