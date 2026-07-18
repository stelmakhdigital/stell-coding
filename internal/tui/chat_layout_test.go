package tui

import (
	"strings"
	"testing"
	"time"

	"stell/coding-agent/internal/themes"
)

func TestPxToCells(t *testing.T) {
	m := &Model{cellW: 8}
	if got := m.pxToCells(10); got != 1 {
		t.Fatalf("10px @8 → %d want 1", got)
	}
	if got := m.pxToCells(15); got != 2 {
		t.Fatalf("15px @8 → %d want 2", got)
	}
	m.cellW = 0
	if got := m.pxToCells(10); got != 1 {
		t.Fatalf("fallback cellW: got %d", got)
	}
}

func TestUserCardFullWidthBg(t *testing.T) {
	m, _ := testModel(t)
	m.width = 80
	m.cellW = 8
	m.activeTheme = themes.DefaultTheme()
	m.colors = paletteFromTheme(m.activeTheme)

	c := card{kind: cardUser, body: "короткий\nдлиннее сообщение пользователя здесь"}
	out := m.renderCardForTest(0, c, 80)
	if !strings.Contains(out, "\x1b[48;") {
		t.Fatalf("user card should have bg CSI: %q", out)
	}
	lay := m.chatLayout()
	want := lay.contentW + lay.blockMargin
	for i, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if got := visibleLen(line); got != want {
			t.Fatalf("line %d visibleLen=%d want %d (contentW+blockMargin): %q", i, got, want, line)
		}
	}
}

func TestUserAndBashBlockAligned(t *testing.T) {
	m, _ := testModel(t)
	m.width = 80
	m.cellW = 8
	m.activeTheme = themes.DefaultTheme()
	m.colors = paletteFromTheme(m.activeTheme)

	user := m.renderCardForTest(0, card{kind: cardUser, body: "hi"}, 80)
	bash := m.renderCardForTest(0, card{
		kind: cardBash, toolName: "bash", toolPath: "ls", toolContent: "ok",
		status: cardStatusSuccess, startedAt: time.Now().Add(-time.Second), endedAt: time.Now(),
	}, 80)
	lu := firstNonEmptyVisible(user)
	lb := firstNonEmptyVisible(bash)
	if lu != lb {
		t.Fatalf("user block width %d != bash block width %d", lu, lb)
	}
	lay := m.chatLayout()
	if lu != lay.contentW+lay.blockMargin {
		t.Fatalf("width=%d want contentW+blockMargin=%d", lu, lay.contentW+lay.blockMargin)
	}
}

func firstNonEmptyVisible(s string) int {
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(stripANSIForTest(line)) == "" && visibleLen(line) == 0 {
			continue
		}
		if visibleLen(line) > 0 {
			return visibleLen(line)
		}
	}
	return 0
}
