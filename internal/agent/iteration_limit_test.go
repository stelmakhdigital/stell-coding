package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stelmakhdigital/ai"
	"github.com/stelmakhdigital/ai/provider/mock"
)

func TestIterationLimitEndsWithFinalAnswer(t *testing.T) {
	ls := []ai.ChatEvent{mock.Call("c", "ls", map[string]any{"path": "."}), mock.Done(10, 1)}
	final := []ai.ChatEvent{mock.Token("final summary"), mock.Done(10, 3)}
	mp := mock.New("mock", ls, ls, ls, final)
	ag := newEmptyResponseAgent(t, mp)
	ag.Config.Settings.Agent.MaxToolIterations = 3

	events := make(chan Event, 256)
	go ag.Run(context.Background(), "explore forever", events)

	var wrapUpNotice, limitNotice bool
	done := ""
	for ev := range events {
		switch ev.Type {
		case EventError:
			t.Fatalf("unexpected error: %v", ev.Err)
		case EventNotice:
			if strings.Contains(ev.Notice, "wrap up") {
				wrapUpNotice = true
			}
			if strings.Contains(ev.Notice, "final answer without tools") {
				limitNotice = true
			}
		case EventDone:
			done = ev.StopReason
		}
	}
	if !wrapUpNotice {
		t.Fatal("expected wrap-up steer notice before the limit")
	}
	if !limitNotice {
		t.Fatal("expected final-answer notice at the limit")
	}
	if done != "completed" {
		t.Fatalf("StopReason = %q, want completed", done)
	}
	var last string
	for _, m := range ag.Sessions.BuildMessages() {
		if m.Role == ai.RoleAssistant {
			last = m.Content
		}
	}
	if last != "final summary" {
		t.Fatalf("last assistant message = %q, want final summary", last)
	}
}

func TestNoIterationCapByDefault(t *testing.T) {
	// Больше tool-итераций, чем старый default cap (256): без лимита
	// цикл должен пройти все и завершиться нормально.
	const iterations = 300
	scripts := make([][]ai.ChatEvent, 0, iterations+1)
	for i := 0; i < iterations; i++ {
		scripts = append(scripts, []ai.ChatEvent{mock.Call("c", "ls", map[string]any{"path": "."}), mock.Done(10, 1)})
	}
	scripts = append(scripts, []ai.ChatEvent{mock.Token("all done"), mock.Done(10, 3)})
	mp := mock.New("mock", scripts...)
	ag := newEmptyResponseAgent(t, mp)
	if ag.Config.Settings.MaxToolIterations() != 0 {
		t.Fatalf("default cap = %d, want 0", ag.Config.Settings.MaxToolIterations())
	}

	events := make(chan Event, 64)
	go ag.Run(context.Background(), "explore everything", events)

	toolCalls := 0
	done := ""
	for ev := range events {
		switch ev.Type {
		case EventError:
			t.Fatalf("unexpected error: %v", ev.Err)
		case EventToolCall:
			toolCalls++
		case EventNotice:
			if strings.Contains(ev.Notice, "iteration limit") {
				t.Fatalf("unexpected iteration limit notice: %q", ev.Notice)
			}
		case EventDone:
			done = ev.StopReason
		}
	}
	if toolCalls != iterations {
		t.Fatalf("tool calls = %d, want %d", toolCalls, iterations)
	}
	if done != "completed" {
		t.Fatalf("StopReason = %q, want completed", done)
	}
}

func TestRepeatedReadGetsDedupHint(t *testing.T) {
	read := func(id string) []ai.ChatEvent {
		return []ai.ChatEvent{mock.Call(id, "read", map[string]any{"path": "hello.txt"}), mock.Done(10, 1)}
	}
	final := []ai.ChatEvent{mock.Token("done"), mock.Done(5, 1)}
	mp := mock.New("mock", read("c1"), read("c2"), final)
	ag := newEmptyResponseAgent(t, mp)
	if err := os.WriteFile(filepath.Join(ag.Config.Workspace, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	events := make(chan Event, 256)
	go ag.Run(context.Background(), "read the file twice", events)

	var results []string
	for ev := range events {
		switch ev.Type {
		case EventError:
			t.Fatalf("unexpected error: %v", ev.Err)
		case EventToolResult:
			results = append(results, ev.ToolResult.Content)
		}
	}
	if len(results) != 2 {
		t.Fatalf("tool results = %d, want 2", len(results))
	}
	if strings.Contains(results[0], "[note]") {
		t.Fatalf("first read should not carry the dedup hint: %q", results[0])
	}
	if !strings.Contains(results[1], "already read") {
		t.Fatalf("second read missing dedup hint: %q", results[1])
	}
}
