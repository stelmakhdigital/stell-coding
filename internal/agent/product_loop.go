package agent

import (
	"context"
	"fmt"

	"github.com/stelmakhdigital/stell-ai"
	coreagent "github.com/stelmakhdigital/stell-agent"
	"github.com/stelmakhdigital/stell-agent/hooks"
	"github.com/stelmakhdigital/stell-coding/internal/config"
)

// productLoopState хранит изменяемую политику runOnce для колбэков CoreLoop.
type productLoopState struct {
	a                    *Agent
	events               chan<- Event
	system               string
	maxIterations        int
	wrapUpSteered        bool
	toolSteered          bool
	consecutiveToolErrors int
	seenReads            map[string]bool
}

// wireProductLoop заполняет колбэки Loop, чтобы product runOnce владел только preamble/postamble.
func (a *Agent) wireProductLoop(loop *coreagent.Loop, st *productLoopState) {
	loop.SkipUserAppend = true
	loop.SuppressMaxIterationsDone = true
	loop.BuildSystem = func(context.Context) string { return st.system }
	// Сохраняем сообщения сессии как есть (product исторически пропускал ConvertToLlm).
	loop.ConvertMessages = func(msgs []ai.Message) []ai.Message { return msgs }

	loop.PrepareNextTurn = func(ctx context.Context, iter int) error {
		if st.maxIterations > 0 && !st.wrapUpSteered && iter >= st.maxIterations-wrapUpIterationsLeft {
			st.wrapUpSteered = true
			if err := a.appendHarnessLabel(st.events, wrapUpSteerMessage(st.maxIterations-iter)); err != nil {
				return err
			}
			emit(st.events, Event{Type: EventNotice, Notice: fmt.Sprintf("approaching tool iteration limit (%d); steering model to wrap up", st.maxIterations)})
		}
		a.emitHook(ctx, hooks.MessageStart, map[string]any{
			"sessionId": a.Sessions.Header.ID, "iteration": iter,
		})
		return nil
	}

	loop.PrepareChat = func(ctx context.Context, iter int, base ai.ChatRequest) ai.ChatRequest {
		level := a.ThinkingLevel
		tokens, level := config.ChatTokenBudget(a.Model, a.Config.Settings, level)
		base.MaxTokens = tokens.MaxTokens
		base.ThinkingLevel = level
		base.ThinkingBudget = tokens.ThinkingBudget
		base.SessionID = a.Sessions.Header.ID
		if a.Tools != nil {
			base.Tools = a.Tools.Defs()
		}
		return base
	}

	loop.StreamFn = func(ctx context.Context, req ai.ChatRequest) (<-chan ai.ChatEvent, error) {
		if a.StreamFn != nil {
			return a.StreamFn(ctx, req)
		}
		return chatWithRetry(ctx, mustProvider(a), req, a.retryControl(), a.RetryHook)
	}

	loop.AfterChatError = func(ctx context.Context, err error) (bool, string) {
		if IsMultimodalUnsupportedError(err) {
			if n := a.Sessions.StripImagesFromActiveBranch(); n > 0 {
				return true, "removed unsupported image data from session; retry without images"
			}
		}
		a.saveSession()
		return false, ""
	}

	loop.ProcessStream = func(ctx context.Context, stream <-chan ai.ChatEvent, ch chan<- Event) (coreagent.AssistantTurn, error) {
		return a.productProcessStream(ctx, stream, ch)
	}

	loop.AfterAssistantTurn = func(ctx context.Context, iter int, turn coreagent.AssistantTurn) (coreagent.TurnDecision, string, bool, error) {
		return st.afterAssistant(ctx, iter, turn)
	}

	loop.BeforeToolCall = func(ctx context.Context, call ai.ToolCall) (bool, map[string]any) {
		blocked, args := a.emitToolCallHook(ctx, call)
		if blocked {
			return true, args
		}
		call.Args = args
		a.emitHook(ctx, hooks.ToolExecutionStart, map[string]any{
			"tool": call.Name, "args": call.Args, "callId": call.ID,
		})
		return false, args
	}

	loop.OnToolProgress = func(call ai.ToolCall, partial string) {
		emit(st.events, Event{
			Type:       EventToolProgress,
			ToolResult: &ToolResult{CallID: call.ID, Name: call.Name, Content: partial},
		})
		if a.Hooks != nil && a.Hooks.HasSubscriber(hooks.ToolExecutionUpdate) {
			a.emitHook(context.Background(), hooks.ToolExecutionUpdate, map[string]any{
				"tool": call.Name, "callId": call.ID, "partial": partial,
			})
		}
	}

	loop.AfterToolCall = func(ctx context.Context, call ai.ToolCall, oc coreagent.ToolCallOutcome) coreagent.ToolCallOutcome {
		trErr := ""
		if oc.Err != nil {
			trErr = oc.Err.Error()
		}
		a.emitHook(ctx, hooks.ToolExecutionEnd, map[string]any{
			"tool": call.Name, "callId": call.ID,
			"isError": trErr != "", "error": trErr,
		})
		if call.Name == "read" && trErr == "" {
			key := readCallKey(call.Args)
			if st.seenReads[key] {
				oc.Result.Content += "\n\n[note] This exact file range was already read earlier in this turn. Reuse the previous result instead of re-reading files."
			}
			st.seenReads[key] = true
		}
		a.hookTool(ctx, hooks.ToolResult, call)
		if trErr != "" {
			st.consecutiveToolErrors++
		} else {
			st.consecutiveToolErrors = 0
		}
		return oc
	}

	loop.AfterTools = func(ctx context.Context, iter int, outcomes []coreagent.ToolCallOutcome) (bool, error) {
		a.saveSession()
		if st.consecutiveToolErrors >= toolErrorSteerThreshold {
			if st.toolSteered {
				a.saveSession()
				emit(st.events, Event{Type: EventError, Err: fmt.Errorf("tool loop stuck after %d consecutive errors", st.consecutiveToolErrors)})
				emit(st.events, Event{Type: EventDone, StopReason: "error"})
				return false, nil
			}
			emit(st.events, Event{Type: EventNotice, Notice: "tool calls failing repeatedly; steering model with workspace path hints"})
			if err := a.appendHarnessLabel(st.events, toolErrorSteerMessage(a.Config.Workspace)); err != nil {
				return false, err
			}
			st.consecutiveToolErrors = 0
			st.toolSteered = true
		}
		return true, nil
	}

	loop.SteerMessage = func() (ai.Message, bool) {
		if a.SteerFn == nil {
			return ai.Message{}, false
		}
		msg, imgs, ok := a.SteerFn()
		if !ok || (msg == "" && len(imgs) == 0) {
			return ai.Message{}, false
		}
		msg = PromptWithImages(msg, imgs)
		return BuildUserMessage(msg, imgs, a.Model), true
	}

	loop.ShouldStopAfterTurn = func(ctx context.Context, iter int, turn coreagent.AssistantTurn, outcomes []coreagent.ToolCallOutcome) (bool, error) {
		if a.ShouldStopAfterTurn != nil {
			return a.ShouldStopAfterTurn(ctx, iter, turn, outcomes)
		}
		return false, nil
	}
}

func mustProvider(a *Agent) ai.Provider {
	prov, _, ok := a.Registry.Get(a.Model.Name)
	if !ok {
		return nil
	}
	return prov
}

func (st *productLoopState) afterAssistant(ctx context.Context, iter int, turn coreagent.AssistantTurn) (coreagent.TurnDecision, string, bool, error) {
	a := st.a
	blocks := turn.Message.Blocks
	toolCalls := turn.ToolCalls
	stopReason := turn.StopReason
	usage := turn.Usage

	if stopReason == "toolUse" && len(toolCalls) == 0 && !ai.HasAssistantOutput(blocks) {
		emit(st.events, Event{Type: EventNotice, Notice: "model ended with toolUse but no tool calls; retrying"})
		return coreagent.TurnContinue, "", false, nil
	}

	if !ai.HasAssistantOutput(blocks) && len(toolCalls) == 0 {
		errMsg := "model returned an empty response"
		if stopReason == "length" {
			errMsg += " (output limit reached before any content, finish_reason: length)"
		}
		emit(st.events, Event{Type: EventError, Err: fmt.Errorf("%s", errMsg)})
		emit(st.events, Event{Type: EventDone, Usage: usage, StopReason: "error"})
		a.saveSession()
		return coreagent.TurnAbort, "error", true, nil
	}

	a.emitHook(ctx, hooks.MessageEnd, map[string]any{
		"sessionId": a.Sessions.Header.ID, "iteration": iter,
	})

	if len(toolCalls) == 0 {
		done := "completed"
		switch stopReason {
		case "length":
			done = "truncated"
		case "incomplete":
			done = "incomplete"
		}
		a.saveSession()
		// Loop отправляет Done после опционального слива follow-up внутри цикла.
		return coreagent.TurnDone, done, false, nil
	}

	if stopReason == "length" {
		emit(st.events, Event{Type: EventNotice, Notice: "response hit the output token limit; rejecting its tool calls so the model re-issues them"})
		for _, call := range toolCalls {
			tr := ToolResult{
				CallID: call.ID,
				Name:   call.Name,
				Error:  fmt.Sprintf("Tool call %q was not executed: the response hit the output token limit, so its arguments may be truncated. Re-issue the tool call with complete arguments.", call.Name),
			}
			emit(st.events, Event{Type: EventToolResult, ToolResult: &tr})
			toolMsg := ai.Message{Role: ai.RoleTool, Content: tr.Error, ToolCallID: call.ID, ToolName: call.Name}
			if _, err := a.Sessions.AppendMessage(toolMsg); err != nil {
				return coreagent.TurnAbort, "error", true, err
			}
		}
		a.saveSession()
		return coreagent.TurnContinue, "", false, nil
	}

	return coreagent.TurnExecuteTools, stopReason, false, nil
}

func (a *Agent) productProcessStream(ctx context.Context, stream <-chan ai.ChatEvent, ch chan<- Event) (coreagent.AssistantTurn, error) {
	asstEntryID := ""
	asstStarted := false
	startAssistant := func() error {
		if asstStarted {
			return nil
		}
		id, err := a.Sessions.BeginAssistantMessage()
		if err != nil {
			return err
		}
		asstEntryID = id
		asstStarted = true
		emit(ch, Event{Type: EventMessageStart, Message: ai.Message{Role: ai.RoleAssistant}})
		return nil
	}

	var blockBuilder ai.BlockBuilder
	var toolCalls []ai.ToolCall
	var usage *ai.Usage
	stopReason := ""
	updateChars := 0
	thinkingChars := 0
	streamState := newStreamEmitState()

	fail := func(err error) (coreagent.AssistantTurn, error) {
		emit(ch, Event{Type: EventError, Err: err})
		emit(ch, Event{Type: EventDone, StopReason: "error"})
		return coreagent.AssistantTurn{}, err
	}

	for ev := range stream {
		if ctx.Err() != nil {
			emit(ch, Event{Type: EventDone, StopReason: "cancelled"})
			return coreagent.AssistantTurn{}, ctx.Err()
		}
		switch ev.Type {
		case ai.EventToken:
			if err := startAssistant(); err != nil {
				return fail(err)
			}
			blockBuilder.AppendText(ev.Token)
			if !streamState.textStarted {
				streamState.textStarted = true
				emitPartialUpdate(ch, streamState, blockBuilder, toolCalls, "text_start", streamState.textContentIndex(blockBuilder), "", nil)
			}
			emit(ch, Event{Type: EventToken, Token: ev.Token})
			emitPartialUpdate(ch, streamState, blockBuilder, toolCalls, "text_delta", streamState.textContentIndex(blockBuilder), ev.Token, nil)
			a.patchAssistantBlocks(asstEntryID, blockBuilder.Blocks(), toolCalls, stopReason, "")
			a.maybeMessageUpdateHook(ctx, ev.Token, &updateChars)
		case ai.EventThinking:
			if err := startAssistant(); err != nil {
				return fail(err)
			}
			if ev.Token != "" {
				blockBuilder.AppendThinking(ev.Token)
			}
			if ev.ThinkingSignature != "" {
				blockBuilder.AppendThinkingSignature(ev.ThinkingSignature)
			}
			if ev.Token != "" {
				if !streamState.thinkingStarted {
					streamState.thinkingStarted = true
					emitPartialUpdate(ch, streamState, blockBuilder, toolCalls, "thinking_start", 0, "", nil)
				}
				emit(ch, Event{Type: EventThinkingToken, Thinking: ev.Token})
				emitPartialUpdate(ch, streamState, blockBuilder, toolCalls, "thinking_delta", 0, ev.Token, nil)
			}
			a.patchAssistantBlocks(asstEntryID, blockBuilder.Blocks(), toolCalls, stopReason, "")
			if ev.Token != "" {
				a.maybeThinkingUpdateHook(ctx, ev.Token, &thinkingChars)
			}
		case ai.EventToolCallDelta:
			idx := ev.ToolCallIndex
			tc := &ai.ToolCall{ID: ev.ToolCallID, Name: ev.ToolCallName}
			if !streamState.toolcallStarted[idx] {
				streamState.toolcallStarted[idx] = true
				emitPartialUpdate(ch, streamState, blockBuilder, toolCalls, "toolcall_start", streamState.toolCallContentIndex(blockBuilder, idx), "", tc)
			}
			emit(ch, Event{
				Type:          EventToolCallDelta,
				ToolCallDelta: ev.ToolCallDelta,
				ToolCallIndex: ev.ToolCallIndex,
				ToolCallID:    ev.ToolCallID,
				ToolCallName:  ev.ToolCallName,
			})
			emitPartialUpdate(ch, streamState, blockBuilder, toolCalls, "toolcall_delta", streamState.toolCallContentIndex(blockBuilder, idx), ev.ToolCallDelta, tc)
		case ai.EventToolCall:
			if ev.ToolCall != nil {
				if err := startAssistant(); err != nil {
					return fail(err)
				}
				blockBuilder.AppendToolCall(*ev.ToolCall)
				toolCalls = append(toolCalls, *ev.ToolCall)
				tc := *ev.ToolCall
				emit(ch, Event{Type: EventToolCall, ToolCall: &tc})
				a.patchAssistantBlocks(asstEntryID, blockBuilder.Blocks(), toolCalls, stopReason, "")
			}
		case ai.EventDone:
			usage = ev.Usage
			stopReason = ev.StopReason
		case ai.EventError:
			if asstStarted {
				asst := ai.AssistantMessage(blockBuilder.Blocks(), toolCalls, "error", ev.Err.Error())
				if err := a.Sessions.PatchAssistantMessage(asstEntryID, asst); err != nil {
					return fail(err)
				}
				emit(ch, Event{Type: EventMessage, Message: asst})
			}
			emit(ch, Event{Type: EventError, Err: ev.Err})
			emit(ch, Event{Type: EventDone, StopReason: "error"})
			a.saveSession()
			return coreagent.AssistantTurn{}, ev.Err
		}
	}

	if ctx.Err() != nil {
		emit(ch, Event{Type: EventDone, StopReason: "cancelled"})
		return coreagent.AssistantTurn{}, ctx.Err()
	}

	blocks := blockBuilder.Blocks()
	asst := ai.AssistantMessage(blocks, toolCalls, stopReason, "")
	if asstStarted {
		if err := a.Sessions.PatchAssistantMessage(asstEntryID, asst); err != nil {
			return fail(err)
		}
		emit(ch, Event{Type: EventMessage, Message: asst})
	}

	return coreagent.AssistantTurn{
		Message:    asst,
		ToolCalls:  toolCalls,
		StopReason: stopReason,
		Usage:      usage,
		SkipAppend: true,
	}, nil
}
