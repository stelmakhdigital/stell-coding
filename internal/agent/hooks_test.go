package agent

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stelmakhdigital/ai"
	"stell/agent/hooks"
	"github.com/stelmakhdigital/ai/provider/mock"
)

// hookRecorder собирает имена эмитированных хуков по порядку.
type hookRecorder struct {
	mu    sync.Mutex
	names []string
}

func (r *hookRecorder) attach(bus *hooks.Bus) {
	bus.OnAny(func(_ context.Context, _ *hooks.Ctx, ev *hooks.Event) error {
		r.mu.Lock()
		r.names = append(r.names, ev.Name)
		r.mu.Unlock()
		return nil
	})
}

func (r *hookRecorder) count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, v := range r.names {
		if v == name {
			n++
		}
	}
	return n
}

func TestToolExecutionHooksFire(t *testing.T) {
	ls := []ai.ChatEvent{mock.Call("c1", "ls", map[string]any{"path": "."}), mock.Done(10, 1)}
	final := []ai.ChatEvent{mock.Token("done"), mock.Done(5, 1)}
	mp := mock.New("mock", ls, final)
	ag := newEmptyResponseAgent(t, mp)
	ag.Hooks = hooks.NewBus()
	rec := &hookRecorder{}
	rec.attach(ag.Hooks)

	events := make(chan Event, 128)
	go ag.Run(context.Background(), "list files", events)
	for ev := range events {
		if ev.Type == EventError {
			t.Fatalf("unexpected error: %v", ev.Err)
		}
	}

	if rec.count(hooks.ToolExecutionStart) != 1 {
		t.Fatalf("tool_execution_start fired %d times, want 1", rec.count(hooks.ToolExecutionStart))
	}
	if rec.count(hooks.ToolExecutionEnd) != 1 {
		t.Fatalf("tool_execution_end fired %d times, want 1", rec.count(hooks.ToolExecutionEnd))
	}
	if rec.count(hooks.ToolCall) != 1 || rec.count(hooks.ToolResult) != 1 {
		t.Fatalf("tool_call=%d tool_result=%d, want 1/1", rec.count(hooks.ToolCall), rec.count(hooks.ToolResult))
	}
}

func TestToolCallHookBlocksExecution(t *testing.T) {
	ls := []ai.ChatEvent{mock.Call("c1", "ls", map[string]any{"path": "."}), mock.Done(10, 1)}
	final := []ai.ChatEvent{mock.Token("done"), mock.Done(5, 1)}
	mp := mock.New("mock", ls, final)
	ag := newEmptyResponseAgent(t, mp)
	ag.Hooks = hooks.NewBus()
	ag.Hooks.On(hooks.ToolCall, func(_ context.Context, _ *hooks.Ctx, ev *hooks.Event) error {
		ev.Block = true
		return nil
	})

	events := make(chan Event, 128)
	go ag.Run(context.Background(), "list files", events)
	blockedResult := ""
	for ev := range events {
		if ev.Type == EventError {
			t.Fatalf("unexpected error: %v", ev.Err)
		}
		if ev.Type == EventToolResult && ev.ToolResult != nil {
			blockedResult = ev.ToolResult.Content
		}
	}
	if blockedResult != "blocked by extension hook" {
		t.Fatalf("tool result = %q, want blocked", blockedResult)
	}
}

func TestBeforeAgentStartAppendsSystem(t *testing.T) {
	final := []ai.ChatEvent{mock.Token("hi"), mock.Done(5, 1)}
	mp := mock.New("mock", final)
	ag := newEmptyResponseAgent(t, mp)
	ag.Hooks = hooks.NewBus()
	ag.Hooks.On(hooks.BeforeAgentStart, func(_ context.Context, _ *hooks.Ctx, ev *hooks.Event) error {
		ev.AppendSystemText("respond in haiku")
		return nil
	})

	events := make(chan Event, 64)
	go ag.Run(context.Background(), "hello", events)
	for ev := range events {
		if ev.Type == EventError {
			t.Fatalf("unexpected error: %v", ev.Err)
		}
	}

	reqs := mp.Requests()
	if len(reqs) == 0 {
		t.Fatal("no model requests recorded")
	}
	sys := ""
	for _, m := range reqs[0].Messages {
		if m.Role == ai.RoleSystem {
			sys = m.Content
		}
	}
	if !strings.Contains(sys, "respond in haiku") {
		t.Fatalf("system prompt missing appendSystem text: %q", sys)
	}
}
