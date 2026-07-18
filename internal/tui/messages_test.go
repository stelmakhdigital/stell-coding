package tui

import (
	"strings"
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/skills"
	"github.com/stelmakhdigital/stell-coding/internal/themes"
)

func TestRenderCardBodyToolFull(t *testing.T) {
	m := &Model{
		keys: DefaultKeybindings(),
		lines: []card{{
			kind: cardTool,
			body: strings.Repeat("line\n", 10),
		}},
	}
	out := m.renderCardForTest(0, m.lines[0], 80)
	if strings.Contains(out, "to expand") || strings.Contains(out, "more line") {
		t.Fatalf("should not collapse: %q", out)
	}
	if strings.Count(out, "line") < 10 {
		t.Fatalf("expected full body (10 lines), got %q", out)
	}
}

func TestRenderCardBodySkillFull(t *testing.T) {
	m := &Model{
		keys: DefaultKeybindings(),
		lines: []card{{
			kind:      cardSkill,
			skillName: "hello",
			skillBody: "secret instructions",
			userTail:  "do it now",
		}},
	}
	out := m.renderCardForTest(0, m.lines[0], 80)
	if !strings.Contains(out, "secret instructions") {
		t.Fatal("skill body should always be shown")
	}
	if !strings.Contains(out, "skill hello") || !strings.Contains(out, "do it now") {
		t.Fatalf("got %q", out)
	}
}

func TestThinkingCardMutedItalic(t *testing.T) {
	m, _ := testModel(t)
	m.width = 80
	m.activeTheme = themes.DefaultTheme()
	m.colors = paletteFromTheme(m.activeTheme)
	m.thinkingCollapsed = false

	out := m.renderCardForTest(0, card{kind: cardThinking, body: "ponder this carefully"}, 80)
	if !strings.Contains(out, "\x1b[3m") {
		t.Fatalf("thinking should be italic: %q", out)
	}
	plain := stripANSIForTest(out)
	if !strings.Contains(plain, "thinking:") || !strings.Contains(plain, "ponder this carefully") {
		t.Fatalf("missing content: %q", plain)
	}
	if strings.Contains(out, "to expand") {
		t.Fatalf("should not collapse thinking: %q", out)
	}
}

func TestCardFromUserContentSkill(t *testing.T) {
	block := skills.FormatSkillBlock("demo", "/p/SKILL.md", "/p", "body text", "args here")
	c := cardFromUserContent(block)
	if c.kind != cardSkill || c.skillName != "demo" || c.userTail != "args here" {
		t.Fatalf("got %+v", c)
	}
}
