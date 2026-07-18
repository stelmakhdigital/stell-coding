package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
	"github.com/stelmakhdigital/ai/provider"
	"github.com/stelmakhdigital/ai/provider/mock"
	"stell/agent/session"
	"stell/agent/tools"

	_ "github.com/stelmakhdigital/ai/provider/mock"
)

func TestReadEditFlow(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(target, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}

	mp := mock.New("mock",
		[]ai.ChatEvent{mock.Call("c1", "read", map[string]any{"path": "hello.txt"}), mock.Done(10, 5)},
		[]ai.ChatEvent{mock.Call("c2", "edit", map[string]any{"path": "hello.txt", "oldText": "world", "newText": "stell"}), mock.Done(20, 3)},
		[]ai.ChatEvent{mock.Token("Done."), mock.Done(5, 2)},
	)

	reg := provider.NewRegistry()
	reg.Register(cfg.Models[0], mp)

	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	if err := tools.RegisterBuiltins(rt); err != nil {
		t.Fatal(err)
	}

	sess := session.NewManager(dir)
	sessPath := filepath.Join(dir, "test.jsonl")

	ag := &Agent{
		Config:   cfg,
		Registry: reg,
		Tools:    rt,
		Sessions: sess,
		SessPath: sessPath,
		Model:    cfg.Models[0],
	}

	events := make(chan Event, 64)
	go ag.Run(context.Background(), "update the file", events)

	var toolCalls int
	for ev := range events {
		switch ev.Type {
		case EventToolCall:
			toolCalls++
		case EventError:
			t.Fatalf("agent error: %v", ev.Err)
		}
	}

	if toolCalls != 2 {
		t.Fatalf("expected 2 tool calls, got %d", toolCalls)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello stell" {
		t.Fatalf("file content = %q, want hello stell", string(data))
	}
	if err := sess.Save(sessPath); err != nil {
		t.Fatal(err)
	}
	loaded, err := session.Open(sessPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Header.Version != session.FormatVersion {
		t.Fatalf("session version = %d", loaded.Header.Version)
	}
	if len(loaded.Entries) < 4 {
		t.Fatalf("expected session entries, got %d", len(loaded.Entries))
	}
}

func newEmptyResponseAgent(t *testing.T, mp *mock.Provider) *Agent {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	reg := provider.NewRegistry()
	reg.Register(cfg.Models[0], mp)
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	if err := tools.RegisterBuiltins(rt); err != nil {
		t.Fatal(err)
	}
	return &Agent{
		Config:   cfg,
		Registry: reg,
		Tools:    rt,
		Sessions: session.NewManager(dir),
		Model:    cfg.Models[0],
	}
}

func TestEmptyResponseErrorsImmediately(t *testing.T) {
	empty := []ai.ChatEvent{mock.Done(10, 0)}
	mp := mock.New("mock", empty)
	ag := newEmptyResponseAgent(t, mp)

	events := make(chan Event, 64)
	go ag.Run(context.Background(), "hello", events)

	retries, errors := 0, 0
	for ev := range events {
		switch ev.Type {
		case EventNotice:
			if strings.Contains(ev.Notice, "retrying") {
				retries++
			}
		case EventError:
			errors++
		}
	}
	if retries != 0 {
		t.Fatalf("retry notices = %d, want 0 (no agent-level empty retries)", retries)
	}
	if errors != 1 {
		t.Fatalf("errors = %d, want 1", errors)
	}
	for _, m := range ag.Sessions.BuildMessages() {
		if m.Role == ai.RoleAssistant && strings.TrimSpace(m.Content) != "" {
			t.Fatalf("assistant message recorded for empty response: %+v", m)
		}
	}
}

func badReadScript() []ai.ChatEvent {
	return []ai.ChatEvent{
		mock.Call("c1", "read", map[string]any{"path": "stell/coding-agent/internal/ai/types.go"}),
		mock.Done(10, 1),
	}
}

func TestToolErrorSteerRecovers(t *testing.T) {
	bad := badReadScript()
	ok := []ai.ChatEvent{mock.Token("analysis done"), mock.Done(10, 3)}
	mp := mock.New("mock", bad, bad, bad, ok)
	ag := newEmptyResponseAgent(t, mp)

	events := make(chan Event, 64)
	go ag.Run(context.Background(), "review the project", events)

	var steered bool
	for ev := range events {
		if ev.Type == EventError {
			t.Fatalf("unexpected error: %v", ev.Err)
		}
		if ev.Type == EventNotice && strings.Contains(ev.Notice, "steering model") {
			steered = true
		}
	}
	if !steered {
		t.Fatal("expected steer notice after repeated tool errors")
	}
	var steerFound bool
	for _, e := range ag.Sessions.ActiveBranch() {
		if e.Type == "label" && strings.Contains(e.Message.Content, "[system steer]") {
			steerFound = true
		}
		if e.Type == "message" && e.Message != nil && e.Message.Role == ai.RoleUser && strings.Contains(e.Message.Content, "[system steer]") {
			t.Fatal("harness steer must not be stored as user message")
		}
	}
	if !steerFound {
		t.Fatal("expected harness steer label in session")
	}
}

func TestToolErrorSteerFailsAfterSecondRound(t *testing.T) {
	bad := badReadScript()
	scripts := make([][]ai.ChatEvent, 0, 7)
	for i := 0; i < 6; i++ {
		scripts = append(scripts, bad)
	}
	mp := mock.New("mock", scripts...)
	ag := newEmptyResponseAgent(t, mp)

	events := make(chan Event, 64)
	go ag.Run(context.Background(), "review the project", events)

	var errMsg string
	for ev := range events {
		if ev.Type == EventError && ev.Err != nil {
			errMsg = ev.Err.Error()
		}
	}
	if !strings.Contains(errMsg, "tool loop stuck") {
		t.Fatalf("error = %q, want tool loop stuck", errMsg)
	}
}
