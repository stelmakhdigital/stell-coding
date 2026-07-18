package tui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestWrapTextLongLine(t *testing.T) {
	text := strings.Repeat("word ", 30)
	got := wrapText(text, 20)
	for _, line := range strings.Split(got, "\n") {
		if runewidth.StringWidth(line) > 20 {
			t.Fatalf("line wider than 20: %q (%d)", line, runewidth.StringWidth(line))
		}
	}
}

func TestWrapTextPreservesParagraphs(t *testing.T) {
	got := wrapText("line one\nline two is longer than width", 10)
	if !strings.HasPrefix(got, "line one\n") {
		t.Fatalf("expected paragraph break preserved, got %q", got)
	}
}

func TestWrapTextLongURL(t *testing.T) {
	url := "https://example.com/very/long/path/without/spaces/at/all/in/the/middle"
	got := wrapText(url, 24)
	if !strings.Contains(got, "\n") {
		t.Fatalf("expected hard wrap for long token, got %q", got)
	}
}
