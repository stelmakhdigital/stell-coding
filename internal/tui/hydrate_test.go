package tui

import (
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/agent"
	"github.com/stelmakhdigital/stell-ai"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-ai/provider"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-coding/internal/skills"
	"github.com/stelmakhdigital/stell-agent/tools"
)

func TestCardsFromSession(t *testing.T) {
	dir := t.TempDir()
	sess := session.NewManager(dir)
	_, _ = sess.AppendMessage(ai.Message{Role: ai.RoleUser, Content: "hello"})
	_, _ = sess.AppendMessage(ai.Message{Role: ai.RoleAssistant, Content: "hi"})
	cards := cardsFromSession(sess, nil)
	if len(cards) != 2 {
		t.Fatalf("got %d cards", len(cards))
	}
	if cards[0].kind != cardUser || cards[1].kind != cardAssistant {
		t.Fatalf("kinds: %v %v", cards[0].kind, cards[1].kind)
	}
}

func TestKeybindingsNamespaced(t *testing.T) {
	kb := Keybindings{Bindings: map[string]string{
		normalizeAction("tui.editor.submit"): "enter",
	}}
	if !kb.Matches(actionSubmit, "enter") {
		t.Fatal("expected submit match")
	}
}

func TestAtQueryAtCursor(t *testing.T) {
	q, ok := atQueryAtCursor("see @read")
	if !ok || q != "read" {
		t.Fatalf("got %q ok=%v", q, ok)
	}
}

func TestHydrateSessionSkillBlock(t *testing.T) {
	block := skills.FormatSkillBlock("demo", "/p/SKILL.md", "/p", "instructions", "run")
	dir := t.TempDir()
	sess := session.NewManager(dir)
	_, _ = sess.AppendMessage(ai.Message{Role: ai.RoleUser, Content: block})
	cards := cardsFromSession(sess, nil)
	if len(cards) != 1 || cards[0].kind != cardSkill {
		t.Fatalf("got %+v", cards)
	}
	if cards[0].skillName != "demo" || cards[0].userTail != "run" {
		t.Fatalf("got %+v", cards[0])
	}
}

func TestHydrateSession(t *testing.T) {
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
	_, _ = sess.AppendMessage(ai.Message{Role: ai.RoleUser, Content: "prior"})
	svc := agent.NewService(cfg, reg, rt, sess, "", cfg.Models[0], nil, nil)

	m := NewModel(t.Context(), Options{Service: svc, Config: cfg})
	if len(m.lines) != 1 {
		t.Fatalf("expected hydrated line, got %d", len(m.lines))
	}
}
