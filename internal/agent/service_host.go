package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stelmakhdigital/stell-coding/internal/extensions"
	"github.com/stelmakhdigital/stell-agent/hooks"
)

// ExternalPrompt ставится в очередь, когда расширение вызывает host/agent/send_user_message в idle.
type ExternalPrompt struct {
	Message   string
	DeliverAs string
}

// SetExternalPromptCh подключает TUI/RPC к запуску ходов агента из расширений.
func (s *Service) SetExternalPromptCh(ch chan<- ExternalPrompt) {
	s.mu.Lock()
	s.externalPromptCh = ch
	s.mu.Unlock()
}

func (s *Service) ExtensionSendUserMessage(ctx context.Context, message, deliverAs string) error {
	if s.IsStreaming() {
		switch deliverAs {
		case "followUp":
			return s.FollowUp(message, nil)
		default:
			return s.Steer(message, nil)
		}
	}
	s.mu.Lock()
	ch := s.externalPromptCh
	s.mu.Unlock()
	if ch == nil {
		return fmt.Errorf("agent idle: no external prompt handler")
	}
	select {
	case ch <- ExternalPrompt{Message: message, DeliverAs: deliverAs}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) ExtensionSendMessage(customType, text string, data json.RawMessage) (string, error) {
	id, err := s.Sessions.AppendTypedCustomMessage(customType, text, data)
	if err != nil {
		return "", err
	}
	s.saveSessionIfNeeded()
	return id, nil
}

func (s *Service) ExtensionReload(ctx context.Context) ([]extensions.ReloadStatus, error) {
	if s.Extensions == nil {
		return nil, fmt.Errorf("extensions not configured")
	}
	return s.Extensions.Reload(ctx)
}

func (s *Service) ExtensionAppendEntry(text string) (string, error) {
	return s.AppendCustomEntry(text)
}

func (s *Service) ExtensionAppendTypedEntry(customType, text string, data json.RawMessage, asMessage bool) (string, error) {
	if asMessage {
		return s.AppendTypedCustomMessage(customType, text, data)
	}
	return s.AppendTypedCustomEntry(customType, text, data)
}

func (s *Service) ExtensionSetModel(name string) error {
	return s.SetModelByName(name)
}

func (s *Service) ExtensionGetThinkingLevel() string {
	return s.GetThinkingLevel()
}

func (s *Service) ExtensionSetThinkingLevel(level string) {
	s.SetThinkingLevel(level)
}

func (s *Service) ExtensionSetLabel(label string) error {
	if s.Sessions == nil {
		return fmt.Errorf("no session")
	}
	_, err := s.Sessions.AppendLabel(label)
	if err != nil {
		return err
	}
	s.saveSessionIfNeeded()
	return nil
}

// EmitInputHook запускает input-хук; возвращает возможно изменённый текст и флаг cancel.
func (s *Service) EmitInputHook(ctx context.Context, text string) (string, bool, error) {
	if s.Hooks == nil {
		return text, false, nil
	}
	ev := &hooks.Event{
		Name:      hooks.Input,
		SessionID: s.Sessions.Header.ID,
		Payload:   map[string]any{"text": text},
		Text:      text,
	}
	if err := s.Hooks.Emit(ctx, ev); err != nil {
		return text, false, err
	}
	if ev.Cancel {
		return text, true, nil
	}
	if ev.Text != "" {
		return ev.Text, false, nil
	}
	return text, false, nil
}

func (s *Service) emitAgentHook(ctx context.Context, name string, payload map[string]any) {
	if s.Hooks == nil {
		return
	}
	sid := ""
	if s.Sessions != nil {
		sid = s.Sessions.Header.ID
	}
	_, _ = s.Hooks.EmitNamed(ctx, name, sid, payload)
}
