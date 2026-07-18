package tui

import "testing"

func TestParseKeyModifyOtherKeysEsc(t *testing.T) {
	msg := parseKey("\x1b[27;1;27~")
	if msg.String() != "esc" {
		t.Fatalf("got %q want esc", msg.String())
	}
	if msg.Type != KeyEsc {
		t.Fatalf("type=%v want KeyEsc", msg.Type)
	}
}

func TestParseKeyCtrlCModifyOtherKeys(t *testing.T) {
	msg := parseKey("\x1b[27;5;99~")
	if msg.String() != "ctrl+c" {
		t.Fatalf("got %q", msg.String())
	}
}

func TestParseKeyDecodePrintableInOverlay(t *testing.T) {
	msg := parseKey("\x1b[109u")
	if msg.String() != "m" {
		t.Fatalf("got %q want m", msg.String())
	}
}

func TestParseKeySpaceLegacy(t *testing.T) {
	msg := parseKey(" ")
	if msg.Type != KeySpace {
		t.Fatalf("type=%v want KeySpace", msg.Type)
	}
	if msg.String() != " " {
		t.Fatalf("got %q want single space", msg.String())
	}
}

func TestParseKeySpaceModifyOtherKeys(t *testing.T) {
	msg := parseKey("\x1b[27;1;32~")
	if msg.Type != KeySpace {
		t.Fatalf("type=%v want KeySpace", msg.Type)
	}
	if msg.String() != " " {
		t.Fatalf("got %q want single space", msg.String())
	}
}
