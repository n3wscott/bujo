package collectiondetail

import (
	"fmt"
	"strings"
	"testing"

	"github.com/muesli/reflow/ansi"

	"tableflip.dev/bujo/pkg/glyph"
)

func stripANSIString(s string) string {
	var b strings.Builder
	ansiSeq := false
	for _, r := range s {
		if r == ansi.Marker {
			ansiSeq = true
			continue
		}
		if ansiSeq {
			if ansi.IsTerminator(r) {
				ansiSeq = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func makeBullet(id, label string) Bullet {
	return Bullet{
		ID:     id,
		Label:  label,
		Bullet: glyph.Task,
	}
}

func TestEnsureScrollAccountsForStickyHeaderSpacing(t *testing.T) {
	longLine := "Write an extra long review comment about the storage refactor PR so we can verify wrapping works for nested subtasks that exceed the available width in the detail pane."
	inbox := []Bullet{
		makeBullet("inbox-01", longLine),
		makeBullet("inbox-02", "Email Alex about the demo"),
		makeBullet("inbox-03", "Archive old OKR doc"),
	}

	projects := make([]Bullet, 0, 12)
	for i := 0; i < 12; i++ {
		projects = append(projects, makeBullet(
			fmt.Sprintf("proj-%02d", i),
			fmt.Sprintf("Metrics dashboard polish%02d", i),
		))
	}

	model := NewModel([]Section{
		{ID: "Inbox", Title: "Inbox", Bullets: inbox},
		{ID: "Projects", Title: "Projects", Bullets: projects},
	})
	model.SetSize(60, 9)
	model.Focus()

	if ok := model.focusBulletByID("Projects", "proj-11"); !ok {
		t.Fatalf("expected to focus bullet proj-11")
	}

	target := model.currentLineIndex()
	if target < 0 {
		t.Fatalf("no current line after focusing bullet")
	}
	contentHeight := model.viewportContentHeight()
	if contentHeight <= 0 {
		t.Fatalf("invalid content height %d", contentHeight)
	}
	top := model.scroll
	bottom := top + contentHeight - 1
	if target < top || target > bottom {
		t.Fatalf("target line %d outside viewport [%d,%d] (sticky=%d height=%d)",
			target, top, bottom, model.stickyHeaderHeight(), contentHeight)
	}

	view := model.View()
	plain := stripANSIString(view)
	if !strings.Contains(plain, "Metrics dashboard polish11") {
		t.Fatalf("expected focused bullet to be visible, got:\n%s", plain)
	}
}
