package agent

import (
	"context"

	"github.com/stelmakhdigital/stell-ai"
	coreagent "github.com/stelmakhdigital/stell-agent"
	"github.com/stelmakhdigital/stell-agent/hooks"
	"github.com/stelmakhdigital/stell-coding/internal/config"
)

func (s *Service) runSession(ctx context.Context, firstPrompt string, firstImages []ai.ImageContent, events chan<- Event) {
	s.runAgentTurn(ctx, firstPrompt, firstImages, false, events)
}

func (s *Service) runSessionContinue(ctx context.Context, events chan<- Event) {
	s.runAgentTurn(ctx, "", nil, true, events)
}

func (s *Service) runAgentTurn(ctx context.Context, firstPrompt string, firstImages []ai.ImageContent, continueMode bool, events chan<- Event) {
	defer s.runWg.Done()
	defer func() {
		s.mu.Lock()
		s.streaming = false
		s.cancel = nil
		s.eventSink = nil
		s.mu.Unlock()
		close(events)
	}()

	prompt := firstPrompt
	turnImages := firstImages
	s.emitAgentHook(ctx, hooks.AgentStart, map[string]any{"sessionId": s.Sessions.Header.ID})
	defer func() {
		s.mu.Lock()
		pending := len(s.steerQueue) + len(s.followUpQueue)
		s.mu.Unlock()
		if pending == 0 {
			s.emitAgentHook(context.Background(), hooks.AgentSettled, nil)
		}
	}()
	defer s.emitAgentHook(context.Background(), hooks.AgentEnd, map[string]any{"sessionId": s.Sessions.Header.ID})

	retryAttempt := 0
	for continueMode || prompt != "" || retryAttempt > 0 {
		if ctx.Err() != nil {
			emit(events, Event{Type: EventDone, StopReason: "cancelled"})
			return
		}

		ag := &Agent{
			Config: s.Config, Registry: s.Registry, Tools: s.Tools,
			Sessions: s.Sessions, SessPath: s.SessPath, Model: s.ActiveModelConfig(),
			Catalog: s.Catalog,
			ThinkingLevel:         s.thinkingLevelLocked(),
			UserImages:            turnImages,
			Hooks:                 s.Hooks,
			StreamFn:              s.StreamFn,
			AutoCompactionEnabled: s.AutoCompactionEnabled,
			RetryEnabled:          s.AutoRetryEnabled,
			RetrySettings:         func() config.RetrySettings { return s.Config.Settings.Retry },
			ShouldAbortRetry:      s.TakeAbortRetry,
			SteerFn: func() (string, []ai.ImageContent, bool) {
				msg, imgs := s.takeSteer()
				return msg, imgs, msg != "" || len(imgs) > 0
			},
			FollowUpFn: func() (string, []ai.ImageContent, bool) {
				s.mu.Lock()
				n := len(s.followUpQueue)
				s.mu.Unlock()
				if n == 0 {
					return "", nil, false
				}
				msg, imgs := s.takeFollowUp()
				return msg, imgs, msg != "" || len(imgs) > 0
			},
			ShouldStopAfterTurn: func(ctx context.Context, iter int, turn coreagent.AssistantTurn, outcomes []coreagent.ToolCallOutcome) (bool, error) {
				return s.takeStopAfterTurn(), nil
			},
			RetryHook: func(start bool, info AutoRetryInfo) {
				s.mu.Lock()
				sink := s.eventSink
				s.mu.Unlock()
				if sink == nil {
					return
				}
				cp := info
				if start {
					sink <- Event{Type: EventAutoRetryStart, WillRetry: info.WillRetry, AutoRetry: &cp}
				} else {
					sink <- Event{Type: EventAutoRetryEnd, WillRetry: info.WillRetry, AutoRetry: &cp}
				}
			},
			CompactionEmitter: s.CompactionEmitter,
		}

		inner := make(chan Event, 64)
		go func(p string, cont bool) {
			defer close(inner)
			if cont {
				ag.runContinue(ctx, inner)
			} else {
				ag.runOnce(ctx, p, inner)
			}
		}(prompt, continueMode)
		continueMode = false

		var turnDone Event
		for ev := range inner {
			if ev.Type == EventDone {
				turnDone = ev
				continue
			}
			events <- ev
		}
		s.FlushPendingBash()

		if ctx.Err() != nil {
			emit(events, Event{Type: EventDone, StopReason: "cancelled"})
			return
		}

		if turnDone.StopReason == "error" && s.shouldRetryAssistantError() && retryAttempt < s.maxRetryAttempts() {
			retryAttempt++
			s.Sessions.RemoveLastAssistantOnBranch()
			if !s.waitAssistantRetry(ctx, events, retryAttempt) {
				turnDone.WillRetry = false
				s.RecordUsage(turnDone.Usage)
				events <- turnDone
				return
			}
			prompt = ""
			turnImages = nil
			turnDone = Event{Type: EventDone, StopReason: "error", Usage: turnDone.Usage, WillRetry: true}
			events <- turnDone
			continue
		}

		if turnDone.StopReason != "" {
			s.RecordUsage(turnDone.Usage)
			s.maybeEmitCacheMissNotice(turnDone.Usage)
			events <- turnDone
		}
		return
	}
}

