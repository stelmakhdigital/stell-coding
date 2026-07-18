package tui

import "testing"

func TestDefaultKeybindings(t *testing.T) {
	kb := DefaultKeybindings()
	want := map[string]string{
		actionInterrupt:      "esc",
		actionClear:          "ctrl+c",
		actionDeleteForward:  "ctrl+d",
		actionSuspend:        "ctrl+z",
		actionThinkingToggle: "ctrl+t",
		actionThinkingCycle:  "shift+tab",
	}
	for action, key := range want {
		if !kb.Matches(action, key) {
			t.Fatalf("%s should bind %q, got %q", action, key, kb.Bindings[action])
		}
	}
	if _, ok := kb.Bindings[actionTreeOpen]; ok && kb.Bindings[actionTreeOpen] != "" {
		t.Fatalf("tree should have no default chord, got %q", kb.Bindings[actionTreeOpen])
	}
	for _, key := range []string{"ctrl+v", "alt+v", "shift+insert"} {
		action, ok := kb.ActionForKey(key)
		if !ok || action != actionPasteClipboard {
			t.Fatalf("%s: action=%q ok=%v (KeyMap sync)", key, action, ok)
		}
		if kb.Map == nil {
			t.Fatal("Map should be synced after DefaultKeybindings")
		}
		if a, ok := kb.Map.Lookup(key); !ok || a != actionPasteClipboard {
			t.Fatalf("Map.Lookup(%s)=%q ok=%v", key, a, ok)
		}
	}
	// App actions must win over overlay defaults on shared chords (map iter was flaky).
	for key, wantAction := range map[string]string{
		"ctrl+d": actionDeleteForward,
		"ctrl+t": actionThinkingToggle,
		"ctrl+l": actionModelSelect,
	} {
		got, ok := kb.ActionForKey(key)
		if !ok || got != wantAction {
			t.Fatalf("ActionForKey(%s)=%q ok=%v want %q", key, got, ok, wantAction)
		}
	}
	// ctrl+o is overlay tree filter only (toolsExpand removed); Map keeps overlay binding.
	if a, ok := kb.ActionForKey("ctrl+o"); !ok || a != actionTreeFilterCycle {
		t.Fatalf("ctrl+o should be tree filter cycle, got %q ok=%v", a, ok)
	}
}

func TestEditorAliasesDocumented(t *testing.T) {
	missing := []string{
		"tui.editor.moveUp",
		"tui.editor.moveDown",
		"tui.editor.insertNewline",
		"tui.editor.deleteCharBackward",
	}
	for _, id := range missing {
		if _, ok := actionAliases[id]; ok {
			t.Fatalf("editor-only alias should stay unmapped: %s", id)
		}
	}
	if actionAliases["app.settings.open"] != actionSettingsOpen {
		t.Fatal("expected app.settings.open alias")
	}
	if actionAliases["app.clear"] != actionClear {
		t.Fatal("expected app.clear alias")
	}
	if actionAliases["tui.editor.yank"] != actionEditorYank {
		t.Fatal("yank must not alias to message copy")
	}
}

func TestPendingStripFormat(t *testing.T) {
	if trimOneLine("a\nb", 10) != "a b" {
		t.Fatal("trimOneLine")
	}
}
