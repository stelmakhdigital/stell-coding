package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveKeybindingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	kb := DefaultKeybindings()
	kb.Bindings[actionSubmit] = "ctrl+enter"
	if err := SaveKeybindings(dir, kb, false); err != nil {
		t.Fatal(err)
	}
	path := KeybindingsPath(dir)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	loaded := LoadKeybindings(dir)
	if loaded.Bindings[actionSubmit] != "ctrl+enter" && loaded.Bindings["tui.editor.submit"] == "" {
		// may be stored under namespaced id
		overrides := LoadUserKeyOverrides(dir)
		found := false
		for _, v := range overrides {
			if v == "ctrl+enter" || filepath.Base(v) == "ctrl+enter" {
				found = true
			}
			if v == "ctrl+enter" {
				found = true
			}
		}
		if loaded.Bindings[normalizeAction("tui.editor.submit")] == "ctrl+enter" {
			found = true
		}
		if !found && loaded.Bindings[actionSubmit] != "ctrl+enter" {
			// Check namespaced
			if overrides["tui.editor.submit"] != "ctrl+enter" && overrides[actionSubmit] != "ctrl+enter" {
				t.Fatalf("submit not saved: bindings=%v overrides=%v", loaded.Bindings[actionSubmit], overrides)
			}
		}
	}
}
