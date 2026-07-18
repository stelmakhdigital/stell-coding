package tui

import "testing"

func TestViewportHeightAccountsForFooter(t *testing.T) {
	m, _ := testModel(t)
	m.width, m.height = 80, 24
	m.composer.SetHeight(3)

	base := m.viewportHeight()
	m.attachments = []composerAttachment{newComposerAttachment("photo.png")}
	m.resizeViewport()
	withChip := m.viewportHeight()
	if withChip >= base {
		t.Fatalf("expected smaller viewport with chips: base=%d with=%d", base, withChip)
	}
	if m.footerLines() < 5 {
		t.Fatalf("footer too small: %d", m.footerLines())
	}
}

func TestActionForKeySuperV(t *testing.T) {
	kb := DefaultKeybindings()
	action, ok := kb.ActionForKey("super+v")
	if !ok || action != actionPasteClipboard {
		t.Fatalf("super+v: action=%q ok=%v", action, ok)
	}
}
