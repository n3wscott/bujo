package addtask

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
)

func TestFocusCyclesAcrossFields(t *testing.T) {
	model := newTestModel()
	if model.focus != fieldTaskInput {
		t.Fatalf("expected initial focus on task input")
	}

	model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if model.focus != fieldParentBullet {
		t.Fatalf("expected focus to wrap to parent bullet, got %v", model.focus)
	}

	model.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if model.focus != fieldTaskInput {
		t.Fatalf("expected focus to wrap back to task input, got %v", model.focus)
	}
}

func TestBlurPreventsKeyHandling(t *testing.T) {
	model := newTestModel()
	model.Update(events.BlurMsg{Component: model.id})
	model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if model.focus != fieldTaskInput {
		t.Fatalf("expected focus to remain unchanged while blurred")
	}
}

func TestConfirmResetFlow(t *testing.T) {
	model := newTestModel()
	model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !model.confirmReset {
		t.Fatalf("expected confirm reset to be set")
	}
	model.Update(tea.KeyPressMsg{Text: "n", Code: 'n'})
	if model.confirmReset {
		t.Fatalf("expected confirm reset to be cleared")
	}

	model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	model.Update(tea.KeyPressMsg{Text: "y", Code: 'y'})
	if model.confirmReset {
		t.Fatalf("expected confirm reset to be cleared after confirmation")
	}
	if model.focused {
		t.Fatalf("expected model to blur after confirmation")
	}
}

// newTestModel wires a cache with a single collection + bullet for addtask tests.
func newTestModel() *Model {
	cache := cache.New("addtask-test")
	cache.SetCollections([]collection.Meta{{Name: "Inbox", Type: collection.TypeGeneric}})
	cache.SetSections([]collectiondetail.Section{{
		ID:    "Inbox",
		Title: "Inbox",
		Bullets: []collectiondetail.Bullet{{
			ID:    "b1",
			Label: "First",
		}},
	}})
	return NewModel(cache, Options{InitialCollectionID: "Inbox"})
}
