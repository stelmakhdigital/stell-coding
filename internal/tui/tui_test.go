package tui

import (
	"strings"
	"testing"


	"github.com/stelmakhdigital/stell-coding/internal/agent"
	"github.com/stelmakhdigital/stell-ai"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-ai/provider"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-agent/tools"
)

func TestStreamBufSurvivesModelCopy(t *testing.T) {
	m := Model{
		streamBuf:      &strings.Builder{},
		streamThinkBuf: &strings.Builder{},
	}
	m.streamBuf.WriteString("hello")
	m2 := m
	m2.streamBuf.WriteString(" world")
	if got := m2.streamBuf.String(); got != "hello world" {
		t.Fatalf("got %q", got)
	}
}

func TestThinkingCardResetsBetweenRetries(t *testing.T) {
	m := Model{
		streamBuf:      &strings.Builder{},
		streamThinkBuf: &strings.Builder{},
		thinkStreamIdx: -1,
	}
	m.width, m.height = 100, 40

	think := func(s string) {
		m.applyEvent(agent.Event{Type: agent.EventThinkingToken, Thinking: s})
	}

	// Attempt 1: reasoning only, then an empty-response retry notice.
	think("attempt one")
	m.applyEvent(agent.Event{Type: agent.EventNotice, Notice: "model returned an empty response, retrying (1/2)"})
	// Attempt 2: fresh reasoning.
	think("attempt two")

	var thinkBodies []string
	for _, c := range m.lines {
		if c.kind == cardThinking {
			thinkBodies = append(thinkBodies, c.body)
		}
	}
	if len(thinkBodies) != 2 {
		t.Fatalf("expected 2 thinking cards, got %d: %v", len(thinkBodies), thinkBodies)
	}
	if thinkBodies[0] != "attempt one" {
		t.Fatalf("first card = %q", thinkBodies[0])
	}
	if thinkBodies[1] != "attempt two" {
		t.Fatalf("second card must not accumulate previous attempt, got %q", thinkBodies[1])
	}
}

func TestThinkingCardAfterToolAndRetries(t *testing.T) {
	m := Model{
		streamBuf:      &strings.Builder{},
		streamThinkBuf: &strings.Builder{},
		thinkStreamIdx: -1,
	}
	m.width, m.height = 100, 40

	think := func(parts ...string) {
		for _, p := range parts {
			m.applyEvent(agent.Event{Type: agent.EventThinkingToken, Thinking: p})
		}
	}

	think(`The user wants me to create a file called hello_world.md`)
	m.applyEvent(agent.Event{Type: agent.EventToolCall, ToolCall: &ai.ToolCall{Name: "write"}})
	m.applyEvent(agent.Event{Type: agent.EventToolResult, ToolResult: &agent.ToolResult{Name: "write", Content: "Wrote hello_world.md"}})

	think(`The user said "дальше" which means "next"`)
	m.applyEvent(agent.Event{Type: agent.EventNotice, Notice: "model returned an empty response, retrying (1/2)"})
	think(`retry attempt two`)
	m.applyEvent(agent.Event{Type: agent.EventNotice, Notice: "model returned an empty response, retrying (2/2)"})
	think(`retry attempt three`)

	var thinkBodies []string
	for _, c := range m.lines {
		if c.kind == cardThinking {
			thinkBodies = append(thinkBodies, c.body)
		}
	}
	if len(thinkBodies) != 4 {
		t.Fatalf("expected 4 thinking cards, got %d: %v", len(thinkBodies), thinkBodies)
	}
	want := "retry attempt two"
	if thinkBodies[2] != want {
		t.Fatalf("retry card must start fresh, got %q", thinkBodies[2])
	}
	if thinkBodies[3] != "retry attempt three" {
		t.Fatalf("third retry card = %q", thinkBodies[3])
	}
}

func TestSlashHelp(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	_ = tools.RegisterBuiltins(rt)
	sess := session.NewManager(dir)
	svc := agent.NewService(cfg, reg, rt, sess, "", cfg.Models[0], nil, nil)

	m := NewModel(t.Context(), Options{Service: svc, Config: cfg})
	m.width, m.height = 100, 40
	m.viewport.Width = 100
	m.viewport.Height = 20

	updated, _ := m.Update(KeyMsg{Type: KeyRunes, Runes: []rune{'/'}})
	m2 := updated
	m2.width, m2.height = 100, 40
	m2.viewport.Width = 100
	m2.viewport.Height = 20
	_ = m2.submitWithText("/help", false)
	if len(m2.lines) == 0 {
		t.Fatal("expected help info card")
	}
	if m2.lines[len(m2.lines)-1].kind != cardInfo {
		t.Fatalf("got kind %q", m2.lines[len(m2.lines)-1].kind)
	}
	if m2.composer.Value() != "" {
		t.Fatalf("composer should be cleared after /help, got %q", m2.composer.Value())
	}
	if !strings.Contains(m2.viewport.View(), "pgup/pgdown") {
		t.Fatalf("viewport missing help text: %q", m2.viewport.View())
	}
}

func TestSlashHelpEnterWithMenu(t *testing.T) {
	m, _ := testModel(t)
	m.width, m.height = 100, 40
	m.resizeViewport()

	m.composer.SetValue("/help")
	m.updateSlashMenu()
	if m.slashMenu == nil {
		t.Fatal("expected slash menu for /help")
	}

	updated, _ := m.Update(KeyMsg{Type: KeyEnter})
	m2 := updated
	if m2.composer.Value() != "" {
		t.Fatalf("composer should be cleared, got %q", m2.composer.Value())
	}
	if m2.slashMenu != nil {
		t.Fatal("slash menu should be closed after submit")
	}
}

func TestRenderTreeEmpty(t *testing.T) {
	sess := session.NewManager(t.TempDir())
	out := renderTree(sess)
	if out == "" {
		t.Fatal("expected tree text")
	}
}
