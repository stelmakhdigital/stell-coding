package tui

import (
	"strings"
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/themes"
	tuilib "github.com/stelmakhdigital/stell-tui"
)

const fixtureGFMTable = `Вот тестовая таблица в формате Markdown (HTML-style):

| Имя | Фамилия | Балл | Категория   | Дата_теста  | Результат    |
|-----|---------|------|-------------|------------|--------------|
| Алексей | Смирнов | 95 | Программирование | 2026-07-15 | ✓ Положительный |
| Мария | Иванова | 87 | Механика          | 2026-07-14 | ✓ Положительный |
| Дмитрий| Козлов  | 43 | Физика            | 2026-07-15 | ✗ Отрицательный |`

func TestThinkingCardRendersBoxTable(t *testing.T) {
	m, _ := testModel(t)
	m.width = 100
	m.activeTheme = themes.DefaultTheme()
	m.colors = paletteFromTheme(m.activeTheme)
	m.thinkingCollapsed = false

	out := m.renderCardForTest(0, card{kind: cardThinking, body: fixtureGFMTable}, 96)
	plain := stripANSIForTest(out)
	if !strings.Contains(plain, "thinking:") {
		t.Fatalf("missing thinking prefix: %q", plain)
	}
	if !strings.Contains(plain, "┌") {
		t.Fatalf("thinking card should box GFM table, got:\n%s", plain)
	}
	if strings.Contains(plain, "| Имя |") {
		t.Fatalf("should not leave raw GFM header in thinking:\n%s", plain)
	}
	assertAlignedBoxRows(t, plain)
}

func TestAssistantCardRendersFixtureBoxTable(t *testing.T) {
	m, _ := testModel(t)
	m.width = 100
	m.activeTheme = themes.DefaultTheme()
	m.colors = paletteFromTheme(m.activeTheme)

	out := m.renderCardForTest(0, card{kind: cardAssistant, body: fixtureGFMTable}, 96)
	plain := stripANSIForTest(out)
	if !strings.Contains(plain, "┌") {
		t.Fatalf("assistant card should box table, got:\n%s", plain)
	}
	if strings.Contains(plain, "| Имя |") {
		t.Fatalf("raw GFM header leaked:\n%s", plain)
	}
	assertAlignedBoxRows(t, plain)
}

func assertAlignedBoxRows(t *testing.T, plain string) {
	t.Helper()
	var rows []string
	for _, line := range strings.Split(plain, "\n") {
		if strings.ContainsAny(line, "┌└├│") {
			rows = append(rows, line)
		}
	}
	if len(rows) < 3 {
		t.Fatalf("expected box rows, got %v", rows)
	}
	w := 0
	for _, row := range rows {
		if strings.HasPrefix(strings.TrimLeft(row, " "), "│") || strings.HasPrefix(row, "│") {
			w = tuilib.VisibleLen(row)
			break
		}
	}
	// Сравниваем видимые ширины строк таблицы (leading space от outputPad допустим).
	base := tuilib.VisibleLen(rows[0])
	for i, row := range rows {
		if tuilib.VisibleLen(row) != base {
			t.Fatalf("row %d width %d != %d: %q", i, tuilib.VisibleLen(row), base, row)
		}
	}
	_ = w
}
