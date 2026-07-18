package rpc

import (
	"strings"

	"github.com/stelmakhdigital/stell-coding/internal/agent"
	"github.com/stelmakhdigital/stell-ai"
)

type EventMapper struct {
	messageUpdateActive bool
	textStarted         bool
	thinkingStarted     bool
	toolcallStarted     map[int]bool
	partialText         strings.Builder
	partialThinking     strings.Builder
	runMessages         []map[string]any
}

func NewEventMapper() *EventMapper {
	return &EventMapper{toolcallStarted: map[int]bool{}}
}

func (m *EventMapper) Map(ev agent.Event) []map[string]any {
	switch ev.Type {
	case agent.EventMessageUpdate:
		if ev.MessageUpdate == nil {
			return nil
		}
		m.messageUpdateActive = true
		mu := ev.MessageUpdate
		partial := assistantRPCMessage(mu.Partial)
		delta := map[string]any{
			"type":         mu.EventType,
			"contentIndex": mu.ContentIndex,
			"partial":      partial,
		}
		if mu.Delta != "" {
			delta["delta"] = mu.Delta
		}
		if tc := mu.ToolCall; tc != nil {
			delta["toolCall"] = tc
		}
		return []map[string]any{{
			"type":                  "message_update",
			"message":               partial,
			"assistantMessageEvent": delta,
		}}

	case agent.EventThinkingToken:
		if m.messageUpdateActive {
			return nil
		}
		var out []map[string]any
		if !m.thinkingStarted {
			m.thinkingStarted = true
			out = append(out, m.messageUpdate(map[string]any{
				"type":         "thinking_start",
				"contentIndex": m.thinkingContentIndex(),
				"partial":      m.partialMessage(),
			}))
		}
		m.partialThinking.WriteString(ev.Thinking)
		out = append(out, m.messageUpdate(map[string]any{
			"type":         "thinking_delta",
			"contentIndex": m.thinkingContentIndex(),
			"delta":        ev.Thinking,
			"partial":      m.partialMessage(),
		}))
		return out

	case agent.EventToken:
		if m.messageUpdateActive {
			return nil
		}
		var out []map[string]any
		if !m.textStarted {
			m.textStarted = true
			out = append(out, m.messageUpdate(map[string]any{
				"type":         "text_start",
				"contentIndex": m.textContentIndex(),
				"partial":      m.partialMessage(),
			}))
		}
		m.partialText.WriteString(ev.Token)
		out = append(out, m.messageUpdate(map[string]any{
			"type":         "text_delta",
			"contentIndex": m.textContentIndex(),
			"delta":        ev.Token,
			"partial":      m.partialMessage(),
		}))
		return out

	case agent.EventToolCallDelta:
		if m.messageUpdateActive {
			return nil
		}
		idx := ev.ToolCallIndex
		var out []map[string]any
		if !m.toolcallStarted[idx] {
			m.toolcallStarted[idx] = true
			out = append(out, m.messageUpdate(map[string]any{
				"type":         "toolcall_start",
				"contentIndex": m.toolCallContentIndex(idx),
				"toolCall": map[string]any{
					"id":   ev.ToolCallID,
					"name": ev.ToolCallName,
				},
			}))
		}
		out = append(out, m.messageUpdate(map[string]any{
			"type":         "toolcall_delta",
			"contentIndex": m.toolCallContentIndex(idx),
			"delta":        ev.ToolCallDelta,
			"partial":      m.partialMessage(),
		}))
		return out

	case agent.EventMessage:
		msg := assistantRPCMessage(ev.Message)
		if ev.Message.Role == ai.RoleAssistant {
			m.runMessages = append(m.runMessages, msg)
		} else {
			m.runMessages = append(m.runMessages, agentMessage(ev.Message))
		}
		var out []map[string]any
		out = append(out, map[string]any{"type": "message_start", "message": msg})
		if ev.Message.Role == ai.RoleAssistant && m.textStarted {
			out = append(out, m.messageUpdate(map[string]any{
				"type":         "text_end",
				"contentIndex": m.textContentIndex(),
				"content":      ev.Message.Content,
				"partial":      msg,
			}))
			m.textStarted = false
			m.partialText.Reset()
		}
		if m.thinkingStarted {
			out = append(out, m.messageUpdate(map[string]any{
				"type":         "thinking_end",
				"contentIndex": m.thinkingContentIndex(),
				"content":      m.partialThinking.String(),
				"partial":      msg,
			}))
			m.thinkingStarted = false
			m.partialThinking.Reset()
		}
		out = append(out, map[string]any{"type": "message_end", "message": msg})
		return out

	case agent.EventToolCall:
		if ev.ToolCall == nil {
			return nil
		}
		tc := ev.ToolCall
		var out []map[string]any
		if len(m.toolcallStarted) == 0 {
			out = append(out, m.messageUpdate(map[string]any{
				"type":         "toolcall_start",
				"contentIndex": m.toolCallContentIndex(0),
				"toolCall":     tc,
			}))
		}
		out = append(out, m.messageUpdate(map[string]any{
			"type":         "toolcall_end",
			"contentIndex": m.toolCallContentIndex(0),
			"toolCall":     tc,
			"partial":      assistantRPCMessage(ai.AssistantMessage(nil, []ai.ToolCall{*tc}, "", "")),
		}))
		out = append(out, map[string]any{
			"type":       "tool_execution_start",
			"toolCallId": tc.ID,
			"toolName":   tc.Name,
			"args":       tc.Args,
		})
		m.toolcallStarted = map[int]bool{}
		return out

	case agent.EventToolProgress:
		if ev.ToolResult == nil {
			return nil
		}
		tr := ev.ToolResult
		text := tr.Content
		partialResult := map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
			"details": toolResultDetails(tr),
		}
		return []map[string]any{{
			"type":          "tool_execution_update",
			"toolCallId":    tr.CallID,
			"toolName":      tr.Name,
			"partial":       text,
			"partialResult": partialResult,
		}}

	case agent.EventToolResult:
		if ev.ToolResult == nil {
			return nil
		}
		tr := ev.ToolResult
		text := tr.Content
		if tr.Error != "" {
			text = tr.Error
		}
		partialResult := map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
			"details": toolResultDetails(tr),
		}
		return []map[string]any{{
			"type":          "tool_execution_update",
			"toolCallId":    tr.CallID,
			"toolName":      tr.Name,
			"partial":       text,
			"partialResult": partialResult,
		}, {
			"type":       "tool_execution_end",
			"toolCallId": tr.CallID,
			"toolName":   tr.Name,
			"result": map[string]any{
				"content": []map[string]any{{"type": "text", "text": text}},
			},
			"isError": tr.Error != "",
		}}

	case agent.EventError:
		if ev.Err == nil {
			return nil
		}
		return []map[string]any{{
			"type": "message_update",
			"assistantMessageEvent": map[string]any{
				"type":   "error",
				"reason": "error",
				"error":  ev.Err.Error(),
			},
		}}

	case agent.EventAutoRetryStart:
		return []map[string]any{autoRetryEvent("auto_retry_start", ev)}
	case agent.EventAutoRetryEnd:
		return []map[string]any{autoRetryEvent("auto_retry_end", ev)}
	case agent.EventNotice:
		if ev.Notice == "" {
			return nil
		}
		return []map[string]any{{
			"type":    "session_info",
			"content": ev.Notice,
		}}

	case agent.EventDone:
		out := []map[string]any{{
			"type":      "agent_end",
			"messages":  append([]map[string]any(nil), m.runMessages...),
			"willRetry": ev.WillRetry,
		}}
		if ev.StopReason != "" {
			out[0]["stopReason"] = ev.StopReason
		}
		if ev.Usage != nil {
			out[0]["usage"] = ev.Usage
		}
		m.runMessages = nil
		m.messageUpdateActive = false
		m.textStarted = false
		m.thinkingStarted = false
		m.toolcallStarted = map[int]bool{}
		m.partialText.Reset()
		m.partialThinking.Reset()
		return out
	}
	return nil
}

func autoRetryEvent(typ string, ev agent.Event) map[string]any {
	out := map[string]any{"type": typ, "willRetry": ev.WillRetry}
	if ev.AutoRetry != nil {
		ar := ev.AutoRetry
		out["attempt"] = ar.Attempt
		out["maxAttempts"] = ar.MaxAttempts
		if ar.DelayMs > 0 {
			out["delayMs"] = ar.DelayMs
		}
		if ar.ErrorMessage != "" {
			out["errorMessage"] = ar.ErrorMessage
		}
		if ar.FinalError != "" {
			out["finalError"] = ar.FinalError
		}
		if typ == "auto_retry_end" {
			out["success"] = ar.Success
		}
	}
	return out
}

func (m *EventMapper) messageUpdate(delta map[string]any) map[string]any {
	return map[string]any{
		"type":                  "message_update",
		"message":               m.partialMessage(),
		"assistantMessageEvent": delta,
	}
}

func (m *EventMapper) partialMessage() map[string]any {
	content := []map[string]any{}
	if t := m.partialThinking.String(); t != "" {
		content = append(content, map[string]any{"type": "thinking", "thinking": t})
	}
	if t := m.partialText.String(); t != "" {
		content = append(content, map[string]any{"type": "text", "text": t})
	}
	return map[string]any{
		"role":    "assistant",
		"content": content,
	}
}

func (m *EventMapper) thinkingContentIndex() int { return 0 }

func (m *EventMapper) textContentIndex() int {
	if m.thinkingStarted || m.partialThinking.Len() > 0 {
		return 1
	}
	return 0
}

func (m *EventMapper) toolCallContentIndex(idx int) int {
	base := 0
	if m.thinkingStarted || m.partialThinking.Len() > 0 {
		base++
	}
	if m.textStarted || m.partialText.Len() > 0 {
		base++
	}
	return base + idx
}

func toolResultDetails(tr *agent.ToolResult) map[string]any {
	details := map[string]any{"truncation": nil, "fullOutputPath": nil}
	if tr == nil {
		return details
	}
	if tr.Truncated {
		details["truncation"] = "length"
	}
	if tr.FullOutputPath != "" {
		details["fullOutputPath"] = tr.FullOutputPath
	}
	return details
}

func assistantRPCMessage(msg ai.Message) map[string]any {
	ai.NormalizeMessage(&msg)
	out := map[string]any{
		"role": string(msg.Role),
	}
	if msg.Role == ai.RoleAssistant {
		out["content"] = ai.BlocksToRPCContent(msg.Blocks)
	} else {
		out["content"] = msg.Content
	}
	if msg.StopReason != "" {
		out["stopReason"] = msg.StopReason
	}
	if msg.ErrorMessage != "" {
		out["errorMessage"] = msg.ErrorMessage
	}
	if len(msg.ToolCalls) > 0 {
		out["toolCalls"] = msg.ToolCalls
	}
	if msg.ToolCallID != "" {
		out["toolCallId"] = msg.ToolCallID
	}
	if msg.ToolName != "" {
		out["toolName"] = msg.ToolName
	}
	return out
}

func agentMessage(msg ai.Message) map[string]any {
	if msg.Role == ai.RoleAssistant {
		return assistantRPCMessage(msg)
	}
	ai.NormalizeMessage(&msg)
	return map[string]any{
		"role":    string(msg.Role),
		"content": msg.Content,
	}
}
