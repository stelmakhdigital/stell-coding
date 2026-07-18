package agent

import (
	"context"
	"testing"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
	"github.com/stelmakhdigital/ai/provider"
	"github.com/stelmakhdigital/ai/provider/mock"
	"stell/agent/session"
	"stell/agent/tools"

	_ "github.com/stelmakhdigital/ai/provider/mock"
)

func TestAppendCancelledToolResults(t *testing.T) {
	sess := session.NewManager(t.TempDir())
	ag := &Agent{Sessions: sess}
	events := make(chan Event, 8)
	calls := []ai.ToolCall{
		{ID: "c1", Name: "read"},
		{ID: "c2", Name: "read"},
	}
	ag.appendCancelledToolResults(events, calls, 1)

	var results int
	select {
	case ev := <-events:
		if ev.Type != EventToolResult {
			t.Fatalf("unexpected event type %q", ev.Type)
		}
		results++
	default:
	}
	if results != 1 {
		t.Fatalf("expected 1 cancelled result event, got %d", results)
	}
	msgs := sess.BuildMessages()
	var toolMsgs int
	for _, m := range msgs {
		if m.Role == ai.RoleTool {
			toolMsgs++
			if m.Content != "cancelled" || m.ToolCallID != "c2" {
				t.Fatalf("unexpected tool msg: %+v", m)
			}
		}
	}
	if toolMsgs != 1 {
		t.Fatalf("expected 1 tool message, got %d", toolMsgs)
	}
}

func TestAutoCompactionToggleDuringRun(t *testing.T) {
	enabled := false
	svc := &Service{Config: &config.Config{Settings: config.DefaultSettings()}, autoCompact: &enabled}
	ag := &Agent{
		Config:                svc.Config,
		AutoCompactionEnabled: svc.AutoCompactionEnabled,
	}
	if ag.AutoCompactionEnabled == nil || ag.AutoCompactionEnabled() {
		t.Fatal("expected auto compaction disabled via runtime toggle")
	}
}

func TestNewSessionDuringStreaming(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	mp := mock.New("mock",
		[]ai.ChatEvent{mock.Token("hello"), mock.Done(1, 1)},
	)
	reg := provider.NewRegistry()
	reg.Register(cfg.Models[0], mp)
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	svc := NewService(cfg, reg, rt, session.NewManager(dir), "", cfg.Models[0], nil, nil)
	oldID := svc.Sessions.Header.ID

	events := make(chan Event, 64)
	if err := svc.Prompt(context.Background(), "hi", "", events); err != nil {
		t.Fatal(err)
	}
	drained := make(chan struct{})
	go func() {
		for range events {
		}
		close(drained)
	}()
	_, err := svc.NewSession(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	<-drained
	if svc.Sessions.Header.ID == oldID {
		t.Fatal("session was not replaced")
	}
}
