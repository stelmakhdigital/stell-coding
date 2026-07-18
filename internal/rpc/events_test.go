package rpc

import (
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/agent"
	"github.com/stelmakhdigital/stell-ai"
)

func TestPartialMessageContentArray(t *testing.T) {
	m := NewEventMapper()
	m.partialThinking.WriteString("think")
	m.thinkingStarted = true
	m.partialText.WriteString("hello")

	partial := m.partialMessage()
	content, ok := partial["content"].([]map[string]any)
	if !ok {
		t.Fatalf("content type=%T", partial["content"])
	}
	if len(content) != 2 {
		t.Fatalf("content len=%d want 2", len(content))
	}
	if content[0]["type"] != "thinking" || content[0]["thinking"] != "think" {
		t.Fatalf("thinking block=%+v", content[0])
	}
	if content[1]["type"] != "text" || content[1]["text"] != "hello" {
		t.Fatalf("text block=%+v", content[1])
	}
}

func TestMessageUpdateContentIndexThinkingThenText(t *testing.T) {
	m := NewEventMapper()
	partial := ai.AssistantMessage([]ai.ContentBlock{{Type: ai.BlockTypeThinking, Text: "t"}}, nil, "", "")
	events := m.Map(agent.Event{
		Type: agent.EventMessageUpdate,
		MessageUpdate: &agent.MessageUpdate{EventType: "thinking_delta", ContentIndex: 0, Delta: "t", Partial: partial},
	})
	if len(events) != 1 {
		t.Fatalf("thinking events=%d want 1", len(events))
	}
	events = m.Map(agent.Event{
		Type: agent.EventMessageUpdate,
		MessageUpdate: &agent.MessageUpdate{EventType: "text_start", ContentIndex: 1, Partial: ai.AssistantMessage([]ai.ContentBlock{
			{Type: ai.BlockTypeThinking, Text: "t"},
			{Type: ai.BlockTypeText, Text: "x"},
		}, nil, "", "")},
	})
	textStart, _ := events[0]["assistantMessageEvent"].(map[string]any)
	if contentIndex(textStart) != 1 {
		t.Fatalf("text_start index=%v want 1", textStart["contentIndex"])
	}
}

func contentIndex(m map[string]any) int {
	switch v := m["contentIndex"].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return -1
	}
}

func TestEventMessageUpdateDirectMapping(t *testing.T) {
	m := NewEventMapper()
	partial := ai.AssistantMessage([]ai.ContentBlock{
		{Type: ai.BlockTypeThinking, Text: "plan"},
	}, nil, "", "")
	out := m.Map(agent.Event{
		Type: agent.EventMessageUpdate,
		MessageUpdate: &agent.MessageUpdate{
			EventType:    "thinking_delta",
			ContentIndex: 0,
			Delta:        "an",
			Partial:      partial,
		},
	})
	if len(out) != 1 {
		t.Fatalf("events=%d", len(out))
	}
	ame, _ := out[0]["assistantMessageEvent"].(map[string]any)
	partialMsg, _ := ame["partial"].(map[string]any)
	content, _ := partialMsg["content"].([]map[string]any)
	if len(content) != 1 || content[0]["thinking"] != "plan" {
		t.Fatalf("content=%v", partialMsg["content"])
	}
}

func TestMessageUpdateNoDuplicateDeltas(t *testing.T) {
	m := NewEventMapper()
	partial := ai.AssistantMessage([]ai.ContentBlock{{Type: ai.BlockTypeText, Text: "hi"}}, nil, "", "")
	_ = m.Map(agent.Event{
		Type: agent.EventMessageUpdate,
		MessageUpdate: &agent.MessageUpdate{EventType: "text_delta", ContentIndex: 0, Delta: "hi", Partial: partial},
	})
	dup := m.Map(agent.Event{Type: agent.EventToken, Token: "ignored"})
	if len(dup) != 0 {
		t.Fatalf("expected token fallback suppressed, got %d events", len(dup))
	}
}

func TestAutoRetryStartPayload(t *testing.T) {
	m := NewEventMapper()
	out := m.Map(agent.Event{
		Type:      agent.EventAutoRetryStart,
		WillRetry: true,
		AutoRetry: &agent.AutoRetryInfo{
			Attempt:      1,
			MaxAttempts:  3,
			DelayMs:      2000,
			ErrorMessage: "429 rate limit",
			WillRetry:    true,
		},
	})
	ev := out[0]
	if ev["attempt"] != 1 && ev["attempt"] != float64(1) {
		t.Fatalf("attempt=%v", ev["attempt"])
	}
	if ev["errorMessage"] != "429 rate limit" {
		t.Fatalf("errorMessage=%v", ev["errorMessage"])
	}
}
