package tui

import (
	"strings"
	"testing"
)

func TestScrollPopupFixedHeight(t *testing.T) {
	labels := make([]string, 20)
	for i := range labels {
		labels[i] = "item"
	}

	first := renderScrollPopup("title", labels, 0, popupMaxRows)
	firstLines := stringsCountNewlines(first)
	for _, cursor := range []int{0, 5, 10, 15, 19} {
		popup := renderScrollPopup("title", labels, cursor, popupMaxRows)
		lines := stringsCountNewlines(popup)
		if lines != firstLines {
			t.Fatalf("cursor %d: got %d lines want %d", cursor, lines, firstLines)
		}
		if lines != scrollPopupLineCount(len(labels), popupMaxRows) {
			t.Fatalf("cursor %d: popup lines %d != scrollPopupLineCount %d", cursor, lines, scrollPopupLineCount(len(labels), popupMaxRows))
		}
	}
}

func TestPopupFixedHeightWhenFewItems(t *testing.T) {
	for _, n := range []int{1, 3, 9} {
		labels := make([]string, n)
		for i := range labels {
			labels[i] = "item"
		}
		popup := renderScrollPopup("title", labels, 0, popupMaxRows)
		lines := stringsCountNewlines(popup)
		want := scrollPopupLineCount(n, popupMaxRows)
		if lines != want {
			t.Fatalf("n=%d: popup lines %d want %d", n, lines, want)
		}
	}

	m, _ := testModel(t)
	m.width, m.height = 80, 40
	m.composer.SetHeight(3)
	m.resizeViewport()

	items3 := make([]slashCommand, 3)
	items9 := make([]slashCommand, 9)
	for i := range items9 {
		items9[i] = slashCommand{name: "/cmd" + string(rune('a'+i)), desc: "x"}
		if i < 3 {
			items3[i] = items9[i]
		}
	}
	m.slashMenu = &slashMenuState{index: 0, items: items3}
	m.resizeViewport()
	footer3 := m.footerLines()

	m.slashMenu = &slashMenuState{index: 0, items: items9}
	m.resizeViewport()
	footer9 := m.footerLines()

	if footer3 != footer9 {
		t.Fatalf("footer differs: 3 items=%d 9 items=%d", footer3, footer9)
	}
}

func TestViewHeightMatchesTerminal(t *testing.T) {
	m, _ := testModel(t)
	m.width, m.height = 80, 40
	m.composer.SetHeight(3)
	m.showStartup = false
	m.resizeViewport()
	m.syncViewport()

	assertViewHeight(t, m, "baseline")

	m.composer.SetValue("/")
	m.updateSlashMenu()
	m.resizeViewport()
	assertViewHeight(t, m, "slash menu open")

	m.composer.SetValue("/h")
	m.updateSlashMenu()
	m.resizeViewport()
	assertViewHeight(t, m, "slash menu filtered")

	items := make([]slashCommand, 3)
	for i := range items {
		items[i] = slashCommand{name: "/cmd" + string(rune('a'+i)), desc: "x"}
	}
	m.slashMenu = &slashMenuState{index: 0, items: items}
	m.resizeViewport()
	assertViewHeight(t, m, "slash menu few items")

	m.slashMenu = nil
	m.composer.SetValue("@main")
	m.fileIndex = []string{"main.go", "cmd/main.go", "internal/foo.go", "bar.go"}
	m.updateInlinePicker()
	m.resizeViewport()
	assertViewHeight(t, m, "at picker open")
}

func TestAtPickerShrinksViewport(t *testing.T) {
	m, _ := testModel(t)
	m.width, m.height = 80, 40
	m.composer.SetHeight(3)
	m.resizeViewport()

	baseViewport := m.viewportHeight()
	baseFooter := m.footerLines()

	m.composer.SetValue("@main")
	m.fileIndex = []string{"main.go", "cmd/main.go", "internal/foo.go"}
	m.updateInlinePicker()
	m.resizeViewport()

	if m.footerLines() <= baseFooter {
		t.Fatalf("footer should grow: base=%d now=%d", baseFooter, m.footerLines())
	}
	if m.viewportHeight() >= baseViewport {
		t.Fatalf("viewport should shrink: base=%d now=%d", baseViewport, m.viewportHeight())
	}
	assertLayoutInvariant(t, m)
}

func TestAtPickerCloseRestoresViewport(t *testing.T) {
	m, _ := testModel(t)
	m.width, m.height = 80, 40
	m.composer.SetHeight(3)
	m.resizeViewport()

	baseViewport := m.viewportHeight()
	baseFooter := m.footerLines()

	m.composer.SetValue("@main")
	m.fileIndex = []string{"main.go", "cmd/main.go", "internal/foo.go"}
	m.updateInlinePicker()
	m.resizeViewport()

	m.composer.SetValue("main")
	m.updateInlinePicker()
	m.resizeViewport()

	if m.footerLines() != baseFooter {
		t.Fatalf("footer should restore: base=%d now=%d", baseFooter, m.footerLines())
	}
	if m.viewportHeight() != baseViewport {
		t.Fatalf("viewport should restore: base=%d now=%d", baseViewport, m.viewportHeight())
	}
	assertLayoutInvariant(t, m)
}

func TestSlashMenuNavPreservesFooterHeight(t *testing.T) {
	m, _ := testModel(t)
	m.width, m.height = 80, 40
	m.composer.SetHeight(3)
	m.resizeViewport()

	items := make([]slashCommand, 20)
	for i := range items {
		items[i] = slashCommand{name: "/cmd" + string(rune('a'+i)), desc: "x"}
	}
	m.slashMenu = &slashMenuState{index: 0, items: items}
	m.resizeViewport()

	baseFooter := m.footerLines()
	for idx := 0; idx < len(items); idx++ {
		m.slashMenu.index = idx
		if m.footerLines() != baseFooter {
			t.Fatalf("index %d: footer changed %d -> %d", idx, baseFooter, m.footerLines())
		}
		popup := m.slashMenuPopup()
		lines := stringsCountNewlines(popup)
		maxVis := m.autocompleteMaxVisible()
		if lines != scrollPopupLineCount(len(items), maxVis) {
			t.Fatalf("index %d: popup lines %d != expected %d", idx, lines, scrollPopupLineCount(len(items), maxVis))
		}
	}
	assertLayoutInvariant(t, m)
}

func assertLayoutInvariant(t *testing.T, m Model) {
	t.Helper()
	total := m.headerLines() + m.viewportHeight() + m.footerLines()
	if total != m.height {
		t.Fatalf("layout mismatch: header=%d viewport=%d footer=%d total=%d want height=%d",
			m.headerLines(), m.viewportHeight(), m.footerLines(), total, m.height)
	}
}

func assertViewHeight(t *testing.T, m Model, label string) {
	t.Helper()
	lines := viewLineCount(m.View())
	if lines != m.height {
		t.Fatalf("%s: view lines %d want terminal height %d", label, lines, m.height)
	}
}

func TestScrollPopupStyledPerLineSGR(t *testing.T) {
	muted := colorSeq("245")
	accent := colorSeq("86")
	labels := make([]string, 20)
	for i := range labels {
		labels[i] = "file" + string(rune('a'+i%10)) + ".go"
	}

	for _, cursor := range []int{0, 4, 10, 19} {
		popup := renderScrollPopupStyled("title", labels, cursor, popupMaxRows, muted, accent)
		// DiffEngine path: split into independent lines.
		for _, line := range strings.Split(strings.TrimRight(popup, "\n"), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !strings.HasPrefix(line, "\x1b[") {
				t.Fatalf("cursor %d: line missing open CSI: %q", cursor, line)
			}
			if !strings.HasSuffix(line, reset) {
				t.Fatalf("cursor %d: line missing reset: %q", cursor, line)
			}
		}

		var selected, other string
		for _, line := range strings.Split(popup, "\n") {
			plain := stripANSIForTest(line)
			if strings.HasPrefix(strings.TrimLeft(plain, " "), "> ") {
				selected = line
			} else if strings.HasPrefix(plain, "  file") {
				other = line
			}
		}
		if selected == "" {
			t.Fatalf("cursor %d: no selected line", cursor)
		}
		if !strings.HasPrefix(selected, accent) {
			t.Fatalf("cursor %d: selected should use accent: %q", cursor, selected)
		}
		if other != "" && !strings.HasPrefix(other, muted) {
			t.Fatalf("cursor %d: non-selected should use muted: %q", cursor, other)
		}
	}
}
