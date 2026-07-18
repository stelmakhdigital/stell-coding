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

func doneWith(reason string) ai.ChatEvent {
	return ai.ChatEvent{Type: ai.EventDone, StopReason: reason, Usage: &ai.Usage{InputTokens: 10, OutputTokens: 5}}
}

func TestTruncatedToolCallsAreNotExecuted(t *testing.T) {
	truncatedWrite := []ai.ChatEvent{
		mock.Call("c1", "write", map[string]any{"path": "out.txt", "content": "half of the fi"}),
		doneWith("length"),
	}
	final := []ai.ChatEvent{mock.Token("done"), doneWith("stop")}
	mp := mock.New("mock", truncatedWrite, final)
	ag := newEmptyResponseAgent(t, mp)

	events := make(chan Event, 256)
	go ag.Run(context.Background(), "write the file", events)

	var rejected bool
	for ev := range events {
		switch ev.Type {
		case EventError:
			t.Fatalf("unexpected error: %v", ev.Err)
		case EventToolResult:
			if strings.Contains(ev.ToolResult.Error, "was not executed") {
				rejected = true
			}
		}
	}
	if !rejected {
		t.Fatal("expected truncated tool call to be rejected")
	}
	if _, err := os.Stat(filepath.Join(ag.Config.Workspace, "out.txt")); !os.IsNotExist(err) {
		t.Fatal("truncated write must not create the file")
	}
	var toolMsgFound bool
	for _, m := range ag.Sessions.BuildMessages() {
		if m.Role == ai.RoleTool && strings.Contains(m.Content, "Re-issue the tool call") {
			toolMsgFound = true
		}
	}
	if !toolMsgFound {
		t.Fatal("expected re-issue error tool result in session")
	}
}

func TestTruncatedTextStopsTurn(t *testing.T) {
	partOne := []ai.ChatEvent{mock.Token("part one"), doneWith("length")}
	mp := mock.New("mock", partOne)
	ag := newEmptyResponseAgent(t, mp)

	events := make(chan Event, 256)
	go ag.Run(context.Background(), "explain the project", events)

	done := ""
	for ev := range events {
		switch ev.Type {
		case EventError:
			t.Fatalf("unexpected error: %v", ev.Err)
		case EventDone:
			done = ev.StopReason
		}
	}
	if done != "truncated" {
		t.Fatalf("StopReason = %q, want truncated", done)
	}
	var assistantCount int
	var lastAssistant string
	for _, m := range ag.Sessions.BuildMessages() {
		if m.Role == ai.RoleUser && strings.Contains(m.Content, "cut off mid-text") {
			t.Fatal("unexpected truncation steer user message in session")
		}
		if m.Role == ai.RoleAssistant {
			assistantCount++
			lastAssistant = m.Content
		}
	}
	if assistantCount != 1 {
		t.Fatalf("assistant entries = %d, want 1 in-place entry", assistantCount)
	}
	if lastAssistant != "part one" {
		t.Fatalf("assistant message = %q, want part one", lastAssistant)
	}
	var msgStopReason string
	for _, m := range ag.Sessions.BuildMessages() {
		if m.Role == ai.RoleAssistant {
			msgStopReason = m.StopReason
		}
	}
	if msgStopReason != "length" {
		t.Fatalf("message stopReason = %q, want length", msgStopReason)
	}
}

func TestIncompleteTextStopsTurn(t *testing.T) {
	part := []ai.ChatEvent{mock.Token("partial"), doneWith("incomplete")}
	mp := mock.New("mock", part)
	ag := newEmptyResponseAgent(t, mp)

	events := make(chan Event, 256)
	go ag.Run(context.Background(), "explain", events)

	done := ""
	for ev := range events {
		switch ev.Type {
		case EventError:
			t.Fatalf("unexpected error: %v", ev.Err)
		case EventDone:
			done = ev.StopReason
		}
	}
	if done != "incomplete" {
		t.Fatalf("StopReason = %q, want incomplete", done)
	}
}

func TestEmptyThinkingOnlyTruncated(t *testing.T) {
	thinkingOnly := []ai.ChatEvent{mock.Thinking("Let me wait for"), doneWith("length")}
	mp := mock.New("mock", thinkingOnly)
	ag := newEmptyResponseAgent(t, mp)

	events := make(chan Event, 256)
	go ag.Run(context.Background(), "explain", events)

	done := ""
	for ev := range events {
		switch ev.Type {
		case EventError:
			t.Fatalf("unexpected error: %v", ev.Err)
		case EventDone:
			done = ev.StopReason
		}
	}
	if done != "truncated" {
		t.Fatalf("StopReason = %q, want truncated", done)
	}
	var thinkingPersisted bool
	for _, m := range ag.Sessions.BuildMessages() {
		if m.Role == ai.RoleAssistant && ai.ThinkingFromBlocks(m.Blocks) == "Let me wait for" {
			thinkingPersisted = true
		}
	}
	if !thinkingPersisted {
		t.Fatal("expected thinking block in session")
	}
}

func TestToolUseWithoutCallsRetries(t *testing.T) {
	bad := []ai.ChatEvent{doneWith("toolUse")}
	ok := []ai.ChatEvent{mock.Token("recovered"), doneWith("stop")}
	mp := mock.New("mock", bad, ok)
	ag := newEmptyResponseAgent(t, mp)

	events := make(chan Event, 256)
	go ag.Run(context.Background(), "hello", events)

	done := ""
	for ev := range events {
		switch ev.Type {
		case EventError:
			t.Fatalf("unexpected error: %v", ev.Err)
		case EventDone:
			done = ev.StopReason
		}
	}
	if done != "completed" {
		t.Fatalf("StopReason = %q, want completed", done)
	}
}
