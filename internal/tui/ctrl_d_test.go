package tui

import "testing"

func TestCtrlDDoesNotQuitWhenEmpty(t *testing.T) {
	m, _ := testModel(t)
	m.composer.SetValue("")
	m.attachments = nil
	_, cmd := m.Update(KeyMsg{Type: KeyOther, raw: "ctrl+d"})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, ok := msg.(quitMsg); ok {
				t.Fatal("ctrl+d should not quit when editor empty")
			}
		}
	}
}

func TestCtrlDDeleteForwardWhenNonEmpty(t *testing.T) {
	m, _ := testModel(t)
	m.composer.SetValue("ab")
	if m.composer.ed == nil {
		t.Fatal("expected editor")
	}
	m.composer.ed.HandleInput("\x01") // home
	updated, cmd := m.Update(KeyMsg{Type: KeyOther, raw: "ctrl+d"})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, ok := msg.(quitMsg); ok {
				t.Fatal("should not quit when non-empty")
			}
		}
	}
	if updated.composer.Value() != "b" {
		t.Fatalf("value=%q want b", updated.composer.Value())
	}
}
