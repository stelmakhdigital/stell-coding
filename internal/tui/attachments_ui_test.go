package tui

import "testing"

func TestAddAttachmentDedup(t *testing.T) {
	m := Model{}
	m.addAttachment("foo.png")
	m.addAttachment("foo.png")
	if len(m.attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(m.attachments))
	}
	if m.attachments[0].Kind != attachmentImage {
		t.Fatalf("expected image kind")
	}
}

func TestRemoveAttachment(t *testing.T) {
	m := Model{attachments: []composerAttachment{
		newComposerAttachment("a.png"),
		newComposerAttachment("b.go"),
	}}
	m.attachmentFocus = 1
	m.removeAttachment(0)
	if len(m.attachments) != 1 || m.attachments[0].Path != "b.go" {
		t.Fatalf("unexpected attachments: %+v", m.attachments)
	}
	if m.attachmentFocus != 0 {
		t.Fatalf("focus=%d", m.attachmentFocus)
	}
}

func TestSplitBindingKeys(t *testing.T) {
	keys := splitBindingKeys("ctrl+v, meta+v ,shift+insert")
	if len(keys) != 3 || keys[1] != "meta+v" {
		t.Fatalf("got %v", keys)
	}
}

func TestActionForKeyMultipleBindings(t *testing.T) {
	kb := DefaultKeybindings()
	for _, key := range []string{"ctrl+v", "super+v", "meta+v", "shift+insert"} {
		action, ok := kb.ActionForKey(key)
		if !ok || action != actionPasteClipboard {
			t.Fatalf("key %q: action=%q ok=%v", key, action, ok)
		}
	}
}

func TestRenderAttachmentChipTruncates(t *testing.T) {
	m := Model{width: 80, colors: defaultPalette()}
	att := composerAttachment{Label: "very-long-filename-that-should-be-truncated.png", Kind: attachmentImage}
	chip := m.renderAttachmentChip(att, false)
	if chip == "" {
		t.Fatal("empty chip")
	}
}
