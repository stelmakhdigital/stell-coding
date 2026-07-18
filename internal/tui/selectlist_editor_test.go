package tui

import (
	"testing"

	tuilib "stell/tui"
)

func TestSelectListFilter(t *testing.T) {
	sl := tuilib.NewSelectList([]string{"claude-opus", "gpt-4o", "claude-sonnet"}, nil)
	sl.SetFilter("sonnet")
	if len(sl.Items) != 1 || sl.Items[0] != "claude-sonnet" {
		t.Fatalf("filter sonnet: got %v", sl.Items)
	}
	sl.SetFilter("")
	if len(sl.Items) != 3 {
		t.Fatalf("clear filter: got %v", sl.Items)
	}
}

func TestExitDeletesForwardWhenNonEmpty(t *testing.T) {
	ed := tuilib.NewEditor()
	ed.SetValue("ab")
	ed.HandleInput("\x1b[D") // cursor before 'b'
	ed.DeleteForward()
	if ed.Value() != "a" {
		t.Fatalf("got %q want %q", ed.Value(), "a")
	}
}
