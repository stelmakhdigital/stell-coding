package tui

import (
	"strings"
	"testing"
)

func TestMarkdownPreviewOverlay(t *testing.T) {
	m, _ := testModel(t)
	m.width = 60
	m.height = 20
	m.lines = []card{{kind: cardAssistant, body: "# Hello\n\n| a | b |\n| - | - |\n| 1 | 2 |\n"}}
	m.openMarkdownPreview()
	if m.overlayMode != overlayMarkdownPreview {
		t.Fatalf("overlayMode=%v want overlayMarkdownPreview", m.overlayMode)
	}
	if !strings.Contains(m.overlay, "Markdown preview") {
		t.Fatalf("overlay missing title: %q", m.overlay)
	}
	if !strings.Contains(stripANSIForTest(m.overlay), "┌") && !strings.Contains(m.overlay, "Hello") {
		t.Fatalf("overlay missing content: %q", m.overlay)
	}
	if !m.handleMarkdownPreviewKey("esc") {
		t.Fatal("esc should close")
	}
	if m.overlayMode != overlayNone {
		t.Fatalf("after esc mode=%v", m.overlayMode)
	}
}

func TestMarkdownPagerCommandSetting(t *testing.T) {
	m, cfg := testModel(t)
	cfg.Settings.MarkdownPager = "less -R"
	m.lines = []card{{kind: cardAssistant, body: "hi"}}
	cmd := m.openMarkdownPagerCmd()
	if cmd == nil {
		t.Fatal("expected pager cmd")
	}
}

func TestMarkdownPreviewEmpty(t *testing.T) {
	m, _ := testModel(t)
	m.lines = nil
	m.openMarkdownPreview()
	if m.overlayMode == overlayMarkdownPreview {
		t.Fatal("should not open without assistant message")
	}
}
