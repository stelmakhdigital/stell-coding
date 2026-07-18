package tui

import (
	"strings"
)

// chatLayout хранит боковые отступы истории чата (box insets).
// blockMargin ≈ 10px от окна; textMargin ≈ 15px; innerPad — текст внутри цветных блоков.
type chatLayout struct {
	blockMargin int
	textMargin  int
	innerPad    int
	contentW    int // colored block width (= terminal - 2*blockMargin)
	textW       int // wrap width for text inside blocks / plain messages
}

func (m *Model) pxToCells(px int) int {
	cw := m.cellW
	if cw <= 0 {
		cw = 8
	}
	n := (px + cw/2) / cw
	if n < 1 {
		return 1
	}
	return n
}

func (m *Model) chatLayout() chatLayout {
	bm := m.pxToCells(10)
	tm := m.pxToCells(15)
	if tm < bm {
		tm = bm
	}
	inner := tm - bm
	if m.cfg != nil {
		if op := m.cfg.Settings.OutputPadOrDefault(); op > inner {
			inner = op
		}
	}
	termW := m.width
	if termW <= 0 {
		termW = 80
	}
	cw := termW - 2*bm
	if cw < 20 {
		cw = 20
	}
	textW := cw - 2*inner
	if textW < 10 {
		textW = 10
	}
	return chatLayout{
		blockMargin: bm,
		textMargin:  tm,
		innerPad:    inner,
		contentW:    cw,
		textW:       textW,
	}
}

func indentLines(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}

func padLinesLeft(s string, n int) string {
	return indentLines(s, n)
}

func (m *Model) finishCardLayout(c card, body string, lay chatLayout) string {
	if body == "" {
		return body
	}
	switch c.kind {
	case cardUser, cardSkill, cardTool, cardBash:
		return indentLines(body, lay.blockMargin)
	default:
		return indentLines(body, lay.textMargin)
	}
}

func (m *Model) userMessageBg() string {
	return m.colors.tokenColor("userMessageBg", m.colors.UserBlock)
}

// renderColoredMessageBlock оборачивает текст inner pad и full-width фоном status/user.
func (m *Model) renderColoredMessageBlock(text string, bg string, lay chatLayout) string {
	wrapped := wrapText(text, lay.textW)
	padded := padLinesLeft(wrapped, lay.innerPad)
	return paintLinesWithBg(padded, bg, lay.contentW)
}

// renderCardForTest рендерит карточку с chat layout + внешним отступом (как в истории).
func (m *Model) renderCardForTest(idx int, c card, termWidth int) string {
	prev := m.width
	m.width = termWidth
	defer func() { m.width = prev }()
	lay := m.chatLayout()
	return m.finishCardLayout(c, m.renderCardBody(idx, c, lay), lay)
}
