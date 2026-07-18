package tui

import "testing"

func TestSlashMenuScrollStart(t *testing.T) {
	if got := slashMenuScrollStart(0, 5, 9); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
	if got := slashMenuScrollStart(15, 20, 9); got != 11 {
		t.Fatalf("got %d want 11", got)
	}
	if got := slashMenuScrollStart(2, 20, 9); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}

func TestSlashMenuShrinksViewport(t *testing.T) {
	m, _ := testModel(t)
	m.width, m.height = 80, 40
	m.composer.SetHeight(3)
	m.resizeViewport()

	baseViewport := m.viewportHeight()
	baseFooter := m.footerLines()

	m.slashMenu = &slashMenuState{
		index: 0,
		items: append([]slashCommand(nil), baseSlashCommands...),
	}
	m.resizeViewport()

	if m.footerLines() <= baseFooter {
		t.Fatalf("footer should grow: base=%d now=%d", baseFooter, m.footerLines())
	}
	if m.viewportHeight() >= baseViewport {
		t.Fatalf("viewport should shrink: base=%d now=%d", baseViewport, m.viewportHeight())
	}
	total := m.headerLines() + m.viewportHeight() + m.footerLines()
	if total != m.height {
		t.Fatalf("layout mismatch: header=%d viewport=%d footer=%d total=%d want height=%d",
			m.headerLines(), m.viewportHeight(), m.footerLines(), total, m.height)
	}
}

func TestSlashMenuPopupCapsRows(t *testing.T) {
	m, _ := testModel(t)
	m.width = 80
	items := make([]slashCommand, 20)
	for i := range items {
		items[i] = slashCommand{name: "/cmd" + string(rune('a'+i)), desc: "x"}
	}
	m.slashMenu = &slashMenuState{index: 15, items: items}
	popup := m.slashMenuPopup()
	lines := stringsCountNewlines(popup)
	maxVis := m.autocompleteMaxVisible()
	if lines != scrollPopupLineCount(len(items), maxVis) {
		t.Fatalf("popup lines %d want %d", lines, scrollPopupLineCount(len(items), maxVis))
	}
}

func stringsCountNewlines(s string) int {
	n := 0
	for _, r := range s {
		if r == '\n' {
			n++
		}
	}
	return n
}
