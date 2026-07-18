package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/stelmakhdigital/stell-ai"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-ai/provider"
	"github.com/stelmakhdigital/stell-ai/provider/mock"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-agent/tools"
)

func newCompactService(t *testing.T, mc config.ModelConfig, keepTokens, numMsgs, msgLen int, mp ai.Provider) (*Service, *session.Manager) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{mc},
		Workspace: dir,
	}
	cfg.Settings.Compaction.KeepRecentTokens = keepTokens

	reg := provider.NewRegistry()
	if mp != nil {
		reg.Register(mc, mp)
	}
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	_ = tools.RegisterBuiltins(rt)
	sess := session.NewManager(dir)
	for i := 0; i < numMsgs; i++ {
		role := ai.RoleUser
		if i%2 == 1 {
			role = ai.RoleAssistant
		}
		if _, err := sess.AppendMessage(ai.Message{Role: role, Content: strings.Repeat("m", msgLen)}); err != nil {
			t.Fatal(err)
		}
	}
	return NewService(cfg, reg, rt, sess, "", mc, nil, nil), sess
}

func TestCompactSkipsTinyHistory(t *testing.T) {
	mc := config.ModelConfig{Name: "mock", Provider: "mock"}
	svc, sess := newCompactService(t, mc, 20000, 4, 3, nil)
	info, err := svc.CompactWithInstructions(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if info == nil || info.RemovedMessages != 0 {
		t.Fatalf("expected no compaction, got %+v", info)
	}
	if len(sess.BuildMessages()) != 4 {
		t.Fatalf("expected 4 messages preserved, got %d", len(sess.BuildMessages()))
	}
}

func TestCompactSummarizesOlderHalfOnSmallHistory(t *testing.T) {
	mc := config.ModelConfig{Name: "mock", Provider: "mock"}
	mp := mock.New("mock", []ai.ChatEvent{mock.Token("summary text"), mock.Done(5, 2)})
	svc, sess := newCompactService(t, mc, 20000, 6, 3, mp)

	info, err := svc.CompactWithInstructions(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if info.RemovedMessages != 3 {
		t.Fatalf("removed = %d, want 3 (older half of 6)", info.RemovedMessages)
	}
	msgs := sess.BuildMessages()
	// summary (system) + 3 сохранённых сообщения
	if len(msgs) != 4 {
		t.Fatalf("messages after compact = %d, want 4", len(msgs))
	}
	if msgs[0].Role != ai.RoleSystem || !strings.Contains(msgs[0].Content, "summary text") {
		t.Fatalf("first message should be the summary, got %+v", msgs[0])
	}
}

func TestCompactKeepBudgetCappedByContextBudget(t *testing.T) {
	// Малое локальное окно: budget = 2048 - 512 - 512 = 1024, keep cap = 512.
	// Шесть сообщений ~208 токенов: в cap помещаются только последние 2, 4 суммируются,
	// хотя KeepRecentTokens (20000) сохранил бы всё.
	mc := config.ModelConfig{Name: "mock", Provider: "mock", ContextWindow: 2048, Local: true}
	mp := mock.New("mock", []ai.ChatEvent{mock.Token("summary"), mock.Done(5, 2)})
	svc, sess := newCompactService(t, mc, 20000, 6, 800, mp)

	info, err := svc.CompactWithInstructions(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if info.RemovedMessages != 4 {
		t.Fatalf("removed = %d, want 4", info.RemovedMessages)
	}
	if got := len(sess.BuildMessages()); got != 3 {
		t.Fatalf("messages after compact = %d, want 3 (summary + 2 kept)", got)
	}
}
