package tui

import (
	"strings"
	"testing"
	"time"

	"stell/coding-agent/internal/agent"
	"github.com/stelmakhdigital/ai"
	"stell/agent/session"
)

func newTestEventModel() *Model {
	return &Model{
		streamBuf:      &strings.Builder{},
		streamThinkBuf: &strings.Builder{},
		thinkStreamIdx: -1,
	}
}

func TestApplyEventToolResultUpdatesExistingCard(t *testing.T) {
	m := newTestEventModel()
	m.lines = []card{{
		kind: cardTool, body: "bash: ls", toolName: "bash",
		status: cardStatusPending, startedAt: time.Now().Add(-time.Second),
	}}
	m.applyEvent(agent.Event{
		Type: agent.EventToolResult,
		ToolResult: &agent.ToolResult{
			Name:    "bash",
			Content: "output",
		},
	})
	if len(m.lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(m.lines))
	}
	if !strings.Contains(m.lines[0].body, "bash → output") {
		t.Fatalf("got %q", m.lines[0].body)
	}
	if m.lines[0].status != cardStatusSuccess {
		t.Fatalf("status=%v", m.lines[0].status)
	}
	if m.lines[0].endedAt.Sub(m.lines[0].startedAt) < time.Second {
		t.Fatalf("Took should preserve startedAt, got started=%v ended=%v", m.lines[0].startedAt, m.lines[0].endedAt)
	}
}

func TestApplyEventToolCallThenResultSingleCard(t *testing.T) {
	m := newTestEventModel()
	m.applyEvent(agent.Event{
		Type:     agent.EventToolCall,
		ToolCall: &ai.ToolCall{ID: "c1", Name: "bash", Args: map[string]any{"command": "ls"}},
	})
	m.applyEvent(agent.Event{
		Type: agent.EventToolProgress,
		ToolResult: &agent.ToolResult{
			CallID:  "c1",
			Name:    "bash",
			Content: "partial",
		},
	})
	m.applyEvent(agent.Event{
		Type: agent.EventToolResult,
		ToolResult: &agent.ToolResult{
			CallID:  "c1",
			Name:    "bash",
			Content: "final",
		},
	})
	if len(m.lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(m.lines))
	}
	if !strings.Contains(m.lines[0].body, "bash → final") {
		t.Fatalf("got %q", m.lines[0].body)
	}
}

func TestMultiToolCallIDMatchingPreservesTook(t *testing.T) {
	m := newTestEventModel()
	m.applyEvent(agent.Event{
		Type:     agent.EventToolCall,
		ToolCall: &ai.ToolCall{ID: "a", Name: "read", Args: map[string]any{"path": "a.go"}},
	})
	m.applyEvent(agent.Event{
		Type:     agent.EventToolCall,
		ToolCall: &ai.ToolCall{ID: "b", Name: "bash", Args: map[string]any{"command": "sleep 1"}},
	})
	if len(m.lines) != 2 {
		t.Fatalf("want 2 pending cards, got %d", len(m.lines))
	}
	// Simulate wall time between call and result.
	m.lines[0].startedAt = time.Now().Add(-2 * time.Second)
	m.lines[1].startedAt = time.Now().Add(-1500 * time.Millisecond)

	m.applyEvent(agent.Event{
		Type: agent.EventToolProgress,
		ToolResult: &agent.ToolResult{CallID: "b", Name: "bash", Content: "line1"},
	})
	if m.lines[1].toolContent != "line1" {
		t.Fatalf("progress should update bash card: %q", m.lines[1].toolContent)
	}
	if strings.Contains(m.lines[0].toolContent, "line1") {
		t.Fatal("progress for b must not update card a")
	}

	m.applyEvent(agent.Event{
		Type: agent.EventToolResult,
		ToolResult: &agent.ToolResult{CallID: "a", Name: "read", Content: "package a"},
	})
	m.applyEvent(agent.Event{
		Type: agent.EventToolResult,
		ToolResult: &agent.ToolResult{CallID: "b", Name: "bash", Content: "done"},
	})
	if len(m.lines) != 2 {
		t.Fatalf("want 2 cards (no duplicates), got %d", len(m.lines))
	}
	if m.lines[0].status != cardStatusSuccess || m.lines[1].status != cardStatusSuccess {
		t.Fatalf("statuses: %v %v", m.lines[0].status, m.lines[1].status)
	}
	tookA := m.lines[0].endedAt.Sub(m.lines[0].startedAt)
	tookB := m.lines[1].endedAt.Sub(m.lines[1].startedAt)
	if tookA < time.Second {
		t.Fatalf("read Took too small: %v (startedAt overwritten?)", tookA)
	}
	if tookB < time.Second {
		t.Fatalf("bash Took too small: %v", tookB)
	}
}

func TestCardsFromSessionBashExecutionRole(t *testing.T) {
	dir := t.TempDir()
	sess := session.NewManager(dir)
	_, err := sess.AppendBashEntry("echo hi", "hi\n", session.BashEntryMeta{ExitCode: 0})
	if err != nil {
		t.Fatal(err)
	}
	cards := cardsFromSession(sess, nil)
	if len(cards) != 1 {
		t.Fatalf("got %d cards", len(cards))
	}
	if cards[0].kind != cardBash {
		t.Fatalf("kind=%v want cardBash", cards[0].kind)
	}
	if cards[0].status != cardStatusSuccess {
		t.Fatalf("status=%v", cards[0].status)
	}
	if cards[0].toolPath != "echo hi" {
		t.Fatalf("toolPath=%q", cards[0].toolPath)
	}
}

func TestCardsFromSessionSkipsToolOnlyAssistant(t *testing.T) {
	dir := t.TempDir()
	sess := session.NewManager(dir)
	_, _ = sess.AppendMessage(ai.Message{Role: ai.RoleUser, Content: "run"})
	_, _ = sess.AppendMessage(ai.Message{
		Role:      ai.RoleAssistant,
		ToolCalls: []ai.ToolCall{{Name: "bash", Args: map[string]any{"command": "ls"}}},
	})
	_, _ = sess.AppendMessage(ai.Message{Role: ai.RoleTool, Content: "out", ToolName: "bash"})
	cards := cardsFromSession(sess, nil)
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want user + tool", len(cards))
	}
	if cards[0].kind != cardUser || cards[1].kind != cardTool {
		t.Fatalf("kinds: %v %v", cards[0].kind, cards[1].kind)
	}
}
