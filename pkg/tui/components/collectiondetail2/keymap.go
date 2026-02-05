package collectiondetail2

import "github.com/charmbracelet/bubbles/v2/key"

// keyMap captures navigation and action bindings for detail rows.
type keyMap struct {
	MoveUp             key.Binding
	MoveDown           key.Binding
	PageUp             key.Binding
	PageDown           key.Binding
	MoveTop            key.Binding
	MoveBottom         key.Binding
	Select             key.Binding
	MoveCollection     key.Binding
	MoveFuture         key.Binding
	Complete           key.Binding
	Strike             key.Binding
	SignifyInvestigate key.Binding
	SignifyInspire     key.Binding
	SignifyPriority    key.Binding
	SignifyNone        key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		MoveUp:             key.NewBinding(key.WithKeys("up", "k")),
		MoveDown:           key.NewBinding(key.WithKeys("down", "j")),
		PageUp:             key.NewBinding(key.WithKeys("pgup", "b")),
		PageDown:           key.NewBinding(key.WithKeys("pgdown", "f")),
		MoveTop:            key.NewBinding(key.WithKeys("home", "g")),
		MoveBottom:         key.NewBinding(key.WithKeys("end", "G")),
		Select:             key.NewBinding(key.WithKeys("enter", " ")),
		MoveCollection:     key.NewBinding(key.WithKeys(">")),
		MoveFuture:         key.NewBinding(key.WithKeys("<")),
		Complete:           key.NewBinding(key.WithKeys("x")),
		Strike:             key.NewBinding(key.WithKeys("delete", "backspace")),
		SignifyInvestigate: key.NewBinding(key.WithKeys("?")),
		SignifyInspire:     key.NewBinding(key.WithKeys("!")),
		SignifyPriority:    key.NewBinding(key.WithKeys("*")),
		SignifyNone:        key.NewBinding(key.WithKeys("|")),
	}
}
