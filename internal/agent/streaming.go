package agent

import "github.com/stelmakhdigital/ai"

type streamEmitState struct {
	textStarted     bool
	thinkingStarted bool
	toolcallStarted map[int]bool
}

func newStreamEmitState() *streamEmitState {
	return &streamEmitState{toolcallStarted: map[int]bool{}}
}

func (s *streamEmitState) textContentIndex(blockBuilder ai.BlockBuilder) int {
	if s.thinkingStarted || blockBuilder.HasThinking() {
		return 1
	}
	return 0
}

func (s *streamEmitState) toolCallContentIndex(blockBuilder ai.BlockBuilder, idx int) int {
	base := 0
	if s.thinkingStarted || blockBuilder.HasThinking() {
		base++
	}
	if s.textStarted || blockBuilder.HasText() {
		base++
	}
	return base + idx
}

func emitPartialUpdate(events chan<- Event, state *streamEmitState, blockBuilder ai.BlockBuilder, toolCalls []ai.ToolCall, updateType string, contentIndex int, delta string, toolCall *ai.ToolCall) {
	blocks := blockBuilder.Blocks()
	partial := ai.Message{Role: ai.RoleAssistant, Blocks: blocks, ToolCalls: toolCalls}
	ai.NormalizeMessage(&partial)
	mu := &MessageUpdate{
		EventType:    updateType,
		ContentIndex: contentIndex,
		Delta:        delta,
		Partial:      partial,
		ToolCall:     toolCall,
	}
	emit(events, Event{Type: EventMessageUpdate, MessageUpdate: mu})
}
