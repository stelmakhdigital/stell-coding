package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/stelmakhdigital/stell-ai"
	coreagent "github.com/stelmakhdigital/stell-agent"
	"github.com/stelmakhdigital/stell-agent/hooks"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-agent/tools"
	"github.com/stelmakhdigital/stell-ai/provider"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-coding/internal/contextbuild"
	"github.com/stelmakhdigital/stell-coding/internal/discovery"
)

// wrapUpIterationsLeft — за сколько итераций инструментов до лимита модель
// получает указание завершить работу.
const wrapUpIterationsLeft = 2

// toolErrorSteerThreshold — число подряд ошибок инструментов до авто-стиринга
// модели с подсказками путей workspace (локальные модели часто галлюцинируют пути).
const toolErrorSteerThreshold = 3

type Agent struct {
	Config                *config.Config
	Registry              *provider.Registry
	Tools                 *tools.Runtime
	Sessions              *session.Manager
	SessPath              string
	Model                 config.ModelConfig
	Catalog               *discovery.Catalog
	Hooks                 *hooks.Bus
	UserImages            []ai.ImageContent
	ThinkingLevel         string
	SteerFn               func() (message string, images []ai.ImageContent, ok bool)
	FollowUpFn            func() (message string, images []ai.ImageContent, ok bool)
	ShouldStopAfterTurn   coreagent.ShouldStopAfterTurn
	RetryHook             func(start bool, info AutoRetryInfo)
	CompactionEmitter     func(start bool, reason string, info any)
	AutoCompactionEnabled func() bool
	RetryEnabled          func() bool
	RetrySettings         func() config.RetrySettings
	ShouldAbortRetry      func() bool
	// StreamFn переопределяет provider Chat, если задан (proxy / custom transport).
	StreamFn              coreagent.StreamFn
}

func (a *Agent) Run(ctx context.Context, prompt string, events chan<- Event) {
	defer close(events)
	a.runOnce(ctx, prompt, events)
}

func (a *Agent) runOnce(ctx context.Context, prompt string, events chan<- Event) {
	userMsg := BuildUserMessage(prompt, a.UserImages, a.Model)
	if _, err := a.Sessions.AppendMessage(userMsg); err != nil {
		emit(events, Event{Type: EventError, Err: err})
		emit(events, Event{Type: EventDone, StopReason: "error"})
		return
	}
	emit(events, Event{Type: EventMessage, Message: userMsg})

	a.emitHook(ctx, hooks.TurnStart, map[string]any{
		"sessionId": a.Sessions.Header.ID, "message": prompt,
	})
	defer a.emitHook(ctx, hooks.TurnEnd, map[string]any{"sessionId": a.Sessions.Header.ID})

	appendSystem := a.hookAppendSystem(ctx, hooks.BeforeAgentStart, map[string]any{
		"message": prompt, "sessionId": a.Sessions.Header.ID,
	})

	prov, _, ok := a.Registry.Get(a.Model.Name)
	if !ok {
		emit(events, Event{Type: EventError, Err: fmt.Errorf("model %q not registered", a.Model.Name)})
		emit(events, Event{Type: EventDone, StopReason: "error"})
		return
	}

	system := a.buildSystem(ctx, appendSystem)

	if err := a.maybeAutoCompact(ctx); err != nil {
		emit(events, Event{Type: EventError, Err: err})
		emit(events, Event{Type: EventDone, StopReason: "error"})
		a.saveSession()
		return
	}

	maxIterations := a.Config.Settings.MaxToolIterations()
	st := &productLoopState{
		a:             a,
		events:        events,
		system:        system,
		maxIterations: maxIterations,
		seenReads:     map[string]bool{},
	}
	loop := a.CoreLoop()
	a.wireProductLoop(loop, st)
	loop.RunPrepared(ctx, "", events)

	if loop.LastStopReason == "max_iterations" {
		a.saveSession()
		emit(events, Event{Type: EventNotice, Notice: fmt.Sprintf("max tool iterations (%d) reached; requesting a final answer without tools", maxIterations)})
		a.finalAnswerWithoutTools(ctx, prov, system, events)
	}
}

// runContinue возобновляет цикл агента без добавления нового user-сообщения
// (RunContinue).
func (a *Agent) runContinue(ctx context.Context, events chan<- Event) {
	a.emitHook(ctx, hooks.TurnStart, map[string]any{
		"sessionId": a.Sessions.Header.ID, "message": "", "continue": true,
	})
	defer a.emitHook(ctx, hooks.TurnEnd, map[string]any{"sessionId": a.Sessions.Header.ID})

	appendSystem := a.hookAppendSystem(ctx, hooks.BeforeAgentStart, map[string]any{
		"message": "", "sessionId": a.Sessions.Header.ID, "continue": true,
	})

	prov, _, ok := a.Registry.Get(a.Model.Name)
	if !ok {
		emit(events, Event{Type: EventError, Err: fmt.Errorf("model %q not registered", a.Model.Name)})
		emit(events, Event{Type: EventDone, StopReason: "error"})
		return
	}

	system := a.buildSystem(ctx, appendSystem)

	if err := a.maybeAutoCompact(ctx); err != nil {
		emit(events, Event{Type: EventError, Err: err})
		emit(events, Event{Type: EventDone, StopReason: "error"})
		a.saveSession()
		return
	}

	maxIterations := a.Config.Settings.MaxToolIterations()
	st := &productLoopState{
		a:             a,
		events:        events,
		system:        system,
		maxIterations: maxIterations,
		seenReads:     map[string]bool{},
	}
	loop := a.CoreLoop()
	a.wireProductLoop(loop, st)
	if err := loop.ContinuePrepared(ctx, events); err != nil {
		emit(events, Event{Type: EventError, Err: err})
		emit(events, Event{Type: EventDone, StopReason: "error"})
		return
	}

	if loop.LastStopReason == "max_iterations" {
		a.saveSession()
		emit(events, Event{Type: EventNotice, Notice: fmt.Sprintf("max tool iterations (%d) reached; requesting a final answer without tools", maxIterations)})
		a.finalAnswerWithoutTools(ctx, prov, system, events)
	}
}

func (a *Agent) patchAssistantBlocks(id string, blocks []ai.ContentBlock, toolCalls []ai.ToolCall, stopReason, errMsg string) {
	if id == "" {
		return
	}
	msg := ai.AssistantMessage(blocks, toolCalls, stopReason, errMsg)
	_ = a.Sessions.PatchAssistantMessage(id, msg)
}

func (a *Agent) appendHarnessLabel(events chan<- Event, text string) error {
	if _, err := a.Sessions.AppendLabel(text); err != nil {
		return err
	}
	emit(events, Event{Type: EventLabel, Notice: text})
	return nil
}

// readCallKey идентифицирует вызов read по path и диапазону строк, чтобы повторные
// одинаковые чтения за ход можно пометить для модели.
func readCallKey(args map[string]any) string {
	path, _ := args["path"].(string)
	return fmt.Sprintf("%s|%v|%v", path, args["startLine"], args["endLine"])
}

func wrapUpSteerMessage(left int) string {
	return fmt.Sprintf(`[system steer] You have only %d tool iterations left for this turn.
Stop exploring and wrap up: finish the most important remaining change, then reply with a summary of what was done and what is left.
Do not start new investigations.`, left)
}

// finalAnswerWithoutTools делает последний вызов модели без инструментов, чтобы
// ход завершился сводкой, а не жёсткой ошибкой при достижении лимита
// итераций.
func (a *Agent) finalAnswerWithoutTools(ctx context.Context, prov ai.Provider, system string, events chan<- Event) {
	if err := a.appendHarnessLabel(events, `[system steer] The tool iteration limit for this turn was reached. Tools are disabled now.
Reply with a final answer: summarize what was accomplished, what remains unfinished, and the recommended next steps.`); err != nil {
		emit(events, Event{Type: EventError, Err: err})
		emit(events, Event{Type: EventDone, StopReason: "error"})
		return
	}

	level := a.ThinkingLevel
	tokens, level := config.ChatTokenBudget(a.Model, a.Config.Settings, level)
	req := ai.ChatRequest{
		Model:          modelID(a.Model),
		Messages:       buildModelMessages(system, a.Sessions.BuildMessages()),
		MaxTokens:      tokens.MaxTokens,
		ThinkingLevel:  level,
		ThinkingBudget: tokens.ThinkingBudget,
		SessionID:      a.Sessions.Header.ID,
	}
	stream, err := chatWithRetry(ctx, prov, req, a.retryControl(), a.RetryHook)
	if err != nil {
		emit(events, Event{Type: EventError, Err: err})
		emit(events, Event{Type: EventDone, StopReason: "error"})
		a.saveSession()
		return
	}

	asstEntryID, err := a.Sessions.BeginAssistantMessage()
	if err != nil {
		emit(events, Event{Type: EventError, Err: err})
		emit(events, Event{Type: EventDone, StopReason: "error"})
		return
	}
	emit(events, Event{Type: EventMessageStart, Message: ai.Message{Role: ai.RoleAssistant}})

	var blockBuilder ai.BlockBuilder
	var usage *ai.Usage
	stopReason := ""
	streamState := newStreamEmitState()
	for ev := range stream {
		if ctx.Err() != nil {
			emit(events, Event{Type: EventDone, StopReason: "cancelled"})
			return
		}
		switch ev.Type {
		case ai.EventToken:
			blockBuilder.AppendText(ev.Token)
			if !streamState.textStarted {
				streamState.textStarted = true
				emitPartialUpdate(events, streamState, blockBuilder, nil, "text_start", streamState.textContentIndex(blockBuilder), "", nil)
			}
			emit(events, Event{Type: EventToken, Token: ev.Token})
			emitPartialUpdate(events, streamState, blockBuilder, nil, "text_delta", streamState.textContentIndex(blockBuilder), ev.Token, nil)
			a.patchAssistantBlocks(asstEntryID, blockBuilder.Blocks(), nil, stopReason, "")
		case ai.EventThinking:
			blockBuilder.AppendThinking(ev.Token)
			if ev.Token != "" {
				if !streamState.thinkingStarted {
					streamState.thinkingStarted = true
					emitPartialUpdate(events, streamState, blockBuilder, nil, "thinking_start", 0, "", nil)
				}
				emit(events, Event{Type: EventThinkingToken, Thinking: ev.Token})
				emitPartialUpdate(events, streamState, blockBuilder, nil, "thinking_delta", 0, ev.Token, nil)
			}
			a.patchAssistantBlocks(asstEntryID, blockBuilder.Blocks(), nil, stopReason, "")
		case ai.EventDone:
			usage = ev.Usage
			stopReason = ev.StopReason
		case ai.EventError:
			emit(events, Event{Type: EventError, Err: ev.Err})
			emit(events, Event{Type: EventDone, StopReason: "error"})
			a.saveSession()
			return
		}
	}
	if ctx.Err() != nil {
		emit(events, Event{Type: EventDone, StopReason: "cancelled"})
		return
	}

	blocks := blockBuilder.Blocks()
	content := strings.TrimSpace(ai.TextFromBlocks(blocks))
	if content == "" && !ai.HasAssistantOutput(blocks) {
		content = "(model produced no final summary after reaching the tool iteration limit)"
	}
	asst := ai.AssistantMessage(blocks, nil, stopReason, "")
	if content != "" && ai.TextFromBlocks(blocks) == "" {
		asst.Content = content
	}
	if err := a.Sessions.PatchAssistantMessage(asstEntryID, asst); err != nil {
		emit(events, Event{Type: EventError, Err: err})
		emit(events, Event{Type: EventDone, StopReason: "error"})
		return
	}
	emit(events, Event{Type: EventMessage, Message: asst})
	a.saveSession()
	emit(events, Event{Type: EventDone, Usage: usage, StopReason: "completed"})
}

func (a *Agent) retryControl() retryControl {
	rs := a.Config.Settings.Retry
	if a.RetrySettings != nil {
		rs = a.RetrySettings()
	}
	enabled := rs.Enabled == nil || *rs.Enabled
	if a.RetryEnabled != nil {
		enabled = a.RetryEnabled()
	}
	return retryControl{settings: rs, enabled: enabled, shouldAbort: a.ShouldAbortRetry}
}

func (a *Agent) appendCancelledToolResults(events chan<- Event, calls []ai.ToolCall, from int) {
	for _, call := range calls[from:] {
		tr := ToolResult{CallID: call.ID, Name: call.Name, Content: "cancelled"}
		emit(events, Event{Type: EventToolResult, ToolResult: &tr})
		toolMsg := ai.Message{
			Role:       ai.RoleTool,
			Content:    "cancelled",
			ToolCallID: call.ID,
			ToolName:   call.Name,
		}
		_, _ = a.Sessions.AppendMessage(toolMsg)
	}
	a.saveSession()
}

func toolErrorSteerMessage(workspace string) string {
	ws := strings.TrimSpace(workspace)
	if ws == "" {
		ws = "(unknown)"
	}
	return fmt.Sprintf(`[system steer] Tool calls are failing because paths look wrong.

Workspace root: %s
Use paths relative to the workspace root. Do NOT prefix paths with "stell/".
Do NOT use /workspace/ or any other absolute path outside the workspace root — it does not exist here.
Examples: internal/ai/types.go, internal/provider/registry.go, cmd/stell/main.go

Re-read the user request and continue with correct relative paths.`, ws)
}

func mergeAppendSystem(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "\n\n" + b
}

// emitToolCallHook запускает обработчики tool_call, которые могут заблокировать вызов или
// перезаписать args.
func (a *Agent) emitToolCallHook(ctx context.Context, call ai.ToolCall) (blocked bool, args map[string]any) {
	args = call.Args
	if a.Hooks == nil {
		return false, args
	}
	ev := &hooks.Event{
		Name:      hooks.ToolCall,
		SessionID: a.Sessions.Header.ID,
		Payload:   map[string]any{"tool": call.Name, "callId": call.ID},
		Args:      call.Args,
	}
	if err := a.Hooks.Emit(ctx, ev); err != nil {
		return false, args
	}
	if ev.Block {
		return true, args
	}
	if len(ev.Args) > 0 {
		args = ev.Args
	}
	return false, args
}

func (a *Agent) hookTool(ctx context.Context, hook string, call ai.ToolCall) {
	a.emitHook(ctx, hook, map[string]any{"tool": call.Name, "args": call.Args})
}

func (a *Agent) emitHook(ctx context.Context, hook string, payload map[string]any) {
	if a.Hooks == nil {
		return
	}
	_, _ = a.Hooks.EmitNamed(ctx, hook, a.Sessions.Header.ID, payload)
}

func (a *Agent) maybeMessageUpdateHook(ctx context.Context, token string, charCount *int) {
	if token == "" || !a.Hooks.HasSubscriber(hooks.MessageUpdate) {
		return
	}
	*charCount += len(token)
	if *charCount < 64 {
		return
	}
	*charCount = 0
	a.emitHook(ctx, hooks.MessageUpdate, map[string]any{"delta": token, "type": "text"})
}

func (a *Agent) maybeThinkingUpdateHook(ctx context.Context, token string, charCount *int) {
	if token == "" || !a.Hooks.HasSubscriber(hooks.MessageUpdate) {
		return
	}
	*charCount += len(token)
	if *charCount < 64 {
		return
	}
	*charCount = 0
	a.emitHook(ctx, hooks.MessageUpdate, map[string]any{"delta": token, "type": "thinking"})
}

func (a *Agent) hookAppendSystem(ctx context.Context, hook string, payload map[string]any) string {
	if a.Hooks == nil {
		return ""
	}
	ev, _ := a.Hooks.EmitNamed(ctx, hook, a.Sessions.Header.ID, payload)
	return ev.AppendSystem
}

func (a *Agent) buildSystem(ctx context.Context, appendSystem string) string {
	appendSystem = mergeAppendSystem(appendSystem, a.hookAppendSystem(ctx, hooks.Context, map[string]any{
		"workspace": a.Config.Workspace,
		"sessionId": a.Sessions.Header.ID,
	}))
	opts := contextbuild.Options{AppendSystem: appendSystem}
	if a.Tools != nil {
		opts.Tools = a.Tools.Defs()
	}
	if a.Catalog != nil && a.Catalog.Skills != nil && hasReadTool(opts.Tools) {
		opts.SkillsPrompt = a.Catalog.Skills.PromptXML()
	}
	return contextbuild.BuildSystem(a.Config.GlobalDir, a.Config.Workspace, opts)
}

func hasReadTool(tools []ai.ToolDef) bool {
	for _, t := range tools {
		if t.Name == "read" {
			return true
		}
	}
	return false
}

func (a *Agent) saveSession() {
	if a.SessPath == "" || a.Sessions == nil {
		return
	}
	reportSessionSaveError(a.SessPath, a.Sessions.Save(a.SessPath))
}

func buildModelMessages(system string, history []ai.Message) []ai.Message {
	out := make([]ai.Message, 0, len(history)+1)
	if system != "" {
		out = append(out, ai.Message{Role: ai.RoleSystem, Content: system})
	}
	out = append(out, history...)
	return out
}

func modelID(mc config.ModelConfig) string {
	if mc.Model != "" {
		return mc.Model
	}
	return mc.Name
}

func emit(ch chan<- Event, ev Event) {
	ch <- ev
}

func NewRequestID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "req-" + hex.EncodeToString(b[:])
}
