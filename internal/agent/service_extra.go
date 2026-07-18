package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
	"stell/agent/hooks"
	"stell/agent/session"
)

var ErrSessionSwitchCancelled = fmt.Errorf("session switch cancelled")

func (s *Service) CycleThinkingLevel() string {
	s.mu.Lock()
	cur := s.thinkingLevel
	if cur == "" {
		cur = "off"
	}
	order := config.SupportedThinkingLevels(s.Model)
	idx := 0
	for i, lv := range order {
		if lv == cur {
			idx = (i + 1) % len(order)
			break
		}
	}
	s.thinkingLevel = order[idx]
	level := s.thinkingLevel
	s.mu.Unlock()
	s.emitAgentHook(context.Background(), hooks.ThinkingLevelSelect, map[string]any{"level": level})
	return level
}

func (s *Service) SetSessionName(name string) {
	s.mu.Lock()
	s.sessionName = strings.TrimSpace(name)
	changed := s.sessionName
	s.mu.Unlock()
	s.emitAgentHook(context.Background(), hooks.SessionInfoChanged, map[string]any{
		"name": changed,
	})
}

func (s *Service) GetSessionStats() map[string]any {
	st := s.GetState()
	s.mu.Lock()
	total := s.totalUsage
	last := s.lastUsage
	s.mu.Unlock()

	userMsgs, asstMsgs, toolCalls, toolResults := s.countSessionMessages()

	tokens := map[string]any{
		"input":  total.InputTokens,
		"output": total.OutputTokens,
		"total":  total.InputTokens + total.OutputTokens,
	}
	if total.CacheRead > 0 {
		tokens["cacheRead"] = total.CacheRead
	}
	if total.CacheWrite > 0 {
		tokens["cacheWrite"] = total.CacheWrite
	}

	out := map[string]any{
		"sessionFile":       st.SessionFile,
		"sessionId":         st.SessionID,
		"sessionName":       st.SessionName,
		"userMessages":      userMsgs,
		"assistantMessages": asstMsgs,
		"toolCalls":         toolCalls,
		"toolResults":       toolResults,
		"totalMessages":     st.MessageCount,
		"messageCount":      st.MessageCount,
		"modelName":         st.ModelName,
		"provider":          st.Provider,
		"thinkingLevel":     st.ThinkingLevel,
		"steeringMode":      st.SteeringMode,
		"followUpMode":      st.FollowUpMode,
		"isStreaming":       st.IsStreaming,
		"pendingCount":      st.PendingCount,
		"tokens":            tokens,
	}
	if total.Cost != nil {
		out["cost"] = *total.Cost
	}
	if st.ContextWindow > 0 {
		ctxTokens := st.ContextTokens
		if ctxTokens == 0 && last != nil && last.InputTokens > 0 {
			ctxTokens = last.InputTokens
		}
		usage := map[string]any{
			"tokens":        ctxTokens,
			"contextWindow": st.ContextWindow,
		}
		if ctxTokens > 0 {
			usage["percent"] = ctxTokens * 100 / st.ContextWindow
		}
		out["contextUsage"] = usage
	}
	return out
}

func (s *Service) countSessionMessages() (user, assistant, toolCalls, toolResults int) {
	for _, e := range s.Sessions.Entries {
		if e.Type != "message" || e.Message == nil {
			continue
		}
		switch e.Message.Role {
		case ai.RoleUser:
			user++
		case ai.RoleAssistant:
			assistant++
			toolCalls += len(e.Message.ToolCalls)
		case ai.RoleTool, ai.RoleToolLegacy:
			toolResults++
		}
	}
	return
}

func (s *Service) GetLastAssistantText() string {
	msgs := s.Sessions.BuildMessages()
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == ai.RoleAssistant && msgs[i].Content != "" {
			return msgs[i].Content
		}
	}
	return ""
}

func (s *Service) GetForkMessages(entryID string) ([]map[string]any, error) {
	leaf := s.Sessions.LeafID()
	if err := s.Sessions.SetLeaf(entryID); err != nil {
		return nil, err
	}
	msgs := s.Sessions.BuildMessages()
	_ = s.Sessions.SetLeaf(leaf)
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, map[string]any{
			"role":    string(m.Role),
			"content": m.Content,
		})
	}
	return out, nil
}

func (s *Service) SetAutoCompaction(enabled bool) {
	s.mu.Lock()
	s.autoCompact = &enabled
	s.mu.Unlock()
}

func (s *Service) AutoCompactionEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.autoCompact != nil {
		return *s.autoCompact
	}
	return s.Config.Settings.CompactionEnabled()
}

func (s *Service) SetAutoRetry(enabled bool) {
	s.mu.Lock()
	s.autoRetry = &enabled
	s.mu.Unlock()
}

func (s *Service) AutoRetryEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.autoRetry != nil {
		return *s.autoRetry
	}
	return s.Config.Settings.Retry.Enabled == nil || *s.Config.Settings.Retry.Enabled
}

func (s *Service) AbortRetry() {
	s.mu.Lock()
	s.abortRetry = true
	s.mu.Unlock()
}

func (s *Service) TakeAbortRetry() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.abortRetry {
		s.abortRetry = false
		return true
	}
	return false
}

func (s *Service) SetModelByName(name string) error {
	for _, m := range s.Config.Models {
		if m.Name == name || m.Model == name {
			s.SetModelRecord(m)
			s.emitAgentHook(context.Background(), hooks.ModelSelect, map[string]any{
				"model": m.Name, "sessionId": s.Sessions.Header.ID,
			})
			return nil
		}
	}
	return fmt.Errorf("model %q not found", name)
}

func (s *Service) AvailableModels() []config.ModelConfig {
	models := s.Config.Models
	if len(s.Config.Settings.EnabledModels) == 0 {
		return models
	}
	allowed := map[string]bool{}
	for _, n := range s.Config.Settings.EnabledModels {
		allowed[n] = true
	}
	var out []config.ModelConfig
	for _, m := range models {
		if allowed[m.Name] {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return models
	}
	return out
}

func (s *Service) PopQueuedMessage() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.followUpQueue) > 0 {
		q := s.followUpQueue[0]
		s.followUpQueue = s.followUpQueue[1:]
		return q.Text, true
	}
	if len(s.steerQueue) > 0 {
		q := s.steerQueue[0]
		s.steerQueue = s.steerQueue[1:]
		return q.Text, true
	}
	return "", false
}

func (s *Service) SwitchSession(ctx context.Context, path string) (bool, error) {
	cancelled := false
	if s.IsStreaming() {
		cancelled = s.AbortAndWait(ctx)
	}
	if err := s.EmitBeforeSessionSwitch(ctx, path, "switch"); err != nil {
		return cancelled, err
	}
	if err := s.OpenSession(path); err != nil {
		return cancelled, err
	}
	_ = s.EmitSessionStart()
	return cancelled, nil
}

func (s *Service) EmitBeforeSessionSwitch(ctx context.Context, path, kind string) error {
	if s.Hooks == nil {
		return nil
	}
	ev, err := s.Hooks.EmitNamed(ctx, hooks.SessionBeforeSwitch, s.Sessions.Header.ID, map[string]any{
		"path": path, "kind": kind,
	})
	if err != nil {
		return err
	}
	if ev.Cancel {
		return ErrSessionSwitchCancelled
	}
	return nil
}

// NewSession создаёт новую сессию, опционально связанную с текущей как parent.
func (s *Service) NewSession(ctx context.Context, parentSessionID string) (bool, error) {
	cancelled := false
	if s.IsStreaming() {
		cancelled = s.AbortAndWait(ctx)
	}
	if err := s.EmitBeforeSessionSwitch(ctx, "", "new"); err != nil {
		return cancelled, err
	}
	ws := s.Config.Workspace
	s.mu.Lock()
	s.Sessions = session.NewManager(ws)
	if parentSessionID != "" {
		s.Sessions.Header.ParentSessionID = parentSessionID
	}
	s.SessPath = session.NewSessionPath(s.Config.SessionsRoot(), ws)
	s.mu.Unlock()
	_ = s.EmitSessionStart()
	return cancelled, nil
}
