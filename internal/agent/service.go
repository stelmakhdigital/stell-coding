package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/stelmakhdigital/stell-ai"
	coreagent "github.com/stelmakhdigital/stell-agent"
	"github.com/stelmakhdigital/stell-agent/hooks"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-agent/tools"
	"github.com/stelmakhdigital/stell-ai/provider"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-coding/internal/discovery"
	"github.com/stelmakhdigital/stell-coding/internal/extensions"
)

type QueuedMessage struct {
	Text   string
	Images []ai.ImageContent
}

// Service — агент в рамках сессии со steering/follow-up и abort.
type Service struct {
	Config      *config.Config
	Registry    *provider.Registry
	Tools       *tools.Runtime
	Sessions    *session.Manager
	SessPath    string
	Model       config.ModelConfig
	Catalog     *discovery.Catalog
	Extensions  *extensions.Supervisor
	GrantBroker *extensions.GrantBroker
	Hooks       *hooks.Bus

	mu                 sync.Mutex
	streaming          bool
	cancel             context.CancelFunc
	eventSink          chan<- Event
	steerQueue         []QueuedMessage
	followUpQueue      []QueuedMessage
	steeringMode       string
	followUpMode       string
	thinkingLevel      string
	sessionName        string
	autoCompact        *bool
	autoRetry          *bool
	abortRetry         bool
	pendingAttachments []string
	pendingImages      []ai.ImageContent
	lastUsage          *ai.Usage
	totalUsage         ai.Usage
	bashCancel         context.CancelFunc
	bashMu             sync.Mutex
	pendingBash        []pendingBashRecord
	externalPromptCh   chan<- ExternalPrompt
	CompactionEmitter  func(start bool, reason string, info any)
	// StreamFn переопределяет транспорт LLM (proxy / custom). Env STELL_PROXY_* применяется в NewService при nil.
	StreamFn           coreagent.StreamFn
	stopAfterTurn      bool
	runWg              sync.WaitGroup
}

func NewService(cfg *config.Config, reg *provider.Registry, tools *tools.Runtime, sess *session.Manager, sessPath string, model config.ModelConfig, catalog *discovery.Catalog, ext *extensions.Supervisor) *Service {
	sm := cfg.Settings.SteeringMode
	if sm == "" {
		sm = "one-at-a-time"
	}
	fm := cfg.Settings.FollowUpMode
	if fm == "" {
		fm = "one-at-a-time"
	}
	return &Service{
		Config: cfg, Registry: reg, Tools: tools,
		Sessions: sess, SessPath: sessPath, Model: model,
		Catalog: catalog, Extensions: ext,
		Hooks:        hooks.NewBus(),
		steeringMode: sm, followUpMode: fm,
		thinkingLevel: cfg.Settings.DefaultThinkingLevel,
		StreamFn:      StreamFnFromEnv(),
	}
}

func (s *Service) EmitSessionStart() error {
	_, err := s.Hooks.EmitNamed(context.Background(), hooks.SessionStart, s.Sessions.Header.ID, map[string]any{
		"cwd": s.Config.Workspace,
	})
	return err
}

func (s *Service) EmitBeforeTree(ctx context.Context) {
	s.emitAgentHook(ctx, hooks.SessionBeforeTree, nil)
}

// EmitSessionTree вызывается после построения дерева сессии (RPC get_tree, TUI
// tree overlay).
func (s *Service) EmitSessionTree(ctx context.Context, entries int) {
	s.emitAgentHook(ctx, hooks.SessionTree, map[string]any{"entries": entries})
}

// EmitSessionShutdown вызывается один раз при graceful shutdown, до остановки
// расширений.
func (s *Service) EmitSessionShutdown(ctx context.Context) {
	s.emitAgentHook(ctx, hooks.SessionShutdown, map[string]any{
		"sessionFile": s.SessPath,
	})
}

func (s *Service) GetThinkingLevel() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.thinkingLevel
}

func (s *Service) SetThinkingLevel(level string) {
	s.mu.Lock()
	s.thinkingLevel = level
	s.mu.Unlock()
}

func (s *Service) SetModelRecord(mc config.ModelConfig) {
	s.SetModel(mc)
	_, _ = s.Sessions.AppendModelChange(mc.Name)
	s.saveSessionIfNeeded()
}

type State struct {
	IsStreaming           bool                     `json:"isStreaming"`
	SessionID             string                   `json:"sessionId"`
	SessionFile           string                   `json:"sessionFile,omitempty"`
	SessionName           string                   `json:"sessionName,omitempty"`
	MessageCount          int                      `json:"messageCount"`
	PendingCount          int                      `json:"pendingMessageCount"`
	SteeringMode          string                   `json:"steeringMode"`
	FollowUpMode          string                   `json:"followUpMode"`
	ModelName             string                   `json:"modelName"`
	Provider              string                   `json:"provider"`
	ThinkingLevel         string                   `json:"thinkingLevel,omitempty"`
	AutoCompactionEnabled bool                     `json:"autoCompactionEnabled"`
	InputTokens           int                      `json:"inputTokens,omitempty"`
	OutputTokens          int                      `json:"outputTokens,omitempty"`
	CacheRead             int                      `json:"cacheRead,omitempty"`
	CacheWrite            int                      `json:"cacheWrite,omitempty"`
	CacheHitRate          int                      `json:"cacheHitRate,omitempty"`
	Cost                  *float64                 `json:"cost,omitempty"`
	ContextTokens         int                      `json:"contextTokens,omitempty"`
	ContextWindow         int                      `json:"contextWindow,omitempty"`
	ExtensionShortcuts    []extensions.ShortcutDef `json:"extensionShortcuts,omitempty"`
	ExtensionFlags        []extensions.FlagDef     `json:"extensionFlags,omitempty"`
}

func (s *Service) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buildState()
}

func (s *Service) buildState() State {
	pending := len(s.steerQueue) + len(s.followUpQueue)
	st := State{
		IsStreaming:           s.streaming,
		SessionID:             s.Sessions.Header.ID,
		SessionFile:           s.SessPath,
		SessionName:           s.sessionName,
		MessageCount:          len(s.Sessions.Entries),
		PendingCount:          pending,
		SteeringMode:          s.steeringMode,
		FollowUpMode:          s.followUpMode,
		ModelName:             s.Model.Name,
		Provider:              s.Model.Provider,
		ThinkingLevel:         s.thinkingLevel,
		AutoCompactionEnabled: s.autoCompactEnabledLocked(),
		// Футер ↑↓: накопленное использование сессии. ContextTokens: размер последнего промпта.
		InputTokens:   s.totalUsage.InputTokens,
		OutputTokens:  s.totalUsage.OutputTokens,
		CacheRead:     s.totalUsage.CacheRead,
		CacheWrite:    s.totalUsage.CacheWrite,
		ContextTokens: s.usageInputLocked(),
		ContextWindow: s.Model.ContextWindow,
	}
	if s.totalUsage.Cost != nil {
		c := *s.totalUsage.Cost
		st.Cost = &c
	}
	in := st.ContextTokens + st.CacheRead
	if st.ContextTokens > 0 && st.CacheRead > 0 {
		// Предпочитаем cache hit последнего хода, если доступен.
		if s.lastUsage != nil && s.lastUsage.InputTokens+s.lastUsage.CacheRead > 0 {
			lastIn := s.lastUsage.InputTokens + s.lastUsage.CacheRead
			st.CacheHitRate = s.lastUsage.CacheRead * 100 / lastIn
		} else if in > 0 {
			st.CacheHitRate = st.CacheRead * 100 / in
		}
	}
	if s.Extensions != nil {
		st.ExtensionShortcuts = s.Extensions.Shortcuts()
		st.ExtensionFlags = s.Extensions.Flags()
	}
	return st
}

func (s *Service) autoCompactEnabledLocked() bool {
	if s.autoCompact != nil {
		return *s.autoCompact
	}
	return s.Config.Settings.CompactionEnabled()
}

func (s *Service) usageInputLocked() int {
	if s.lastUsage != nil {
		return s.lastUsage.InputTokens
	}
	return EstimateTokens(s.Sessions.BuildContextEntries())
}

func (s *Service) thinkingLevelLocked() string {
	return s.thinkingLevel
}

func (s *Service) SetPendingImages(images []ai.ImageContent) {
	s.mu.Lock()
	s.pendingImages = append([]ai.ImageContent(nil), images...)
	s.mu.Unlock()
}

func (s *Service) SetPendingAttachments(paths []string) {
	s.mu.Lock()
	s.pendingAttachments = append([]string(nil), paths...)
	s.mu.Unlock()
}

func (s *Service) maybeEmitCacheMissNotice(u *ai.Usage) {
	if u == nil || s.Config == nil || !s.Config.Settings.ShowCacheMiss() {
		return
	}
	if s.Model.Provider != "anthropic" {
		return
	}
	if u.InputTokens < 4096 || u.CacheRead > 0 || u.CacheWrite > 0 {
		return
	}
	s.mu.Lock()
	sink := s.eventSink
	s.mu.Unlock()
	if sink != nil {
		sink <- Event{Type: EventNotice, Notice: "prompt cache miss (no cache read tokens)"}
	}
}

// WireCompactionNotices направляет start/end авто-компакции в поток событий
// как notices, чтобы интерактивные клиенты (TUI) видели компакцию истории
// вместо тихого выполнения.
func (s *Service) WireCompactionNotices() {
	s.CompactionEmitter = func(start bool, reason string, info any) {
		s.mu.Lock()
		sink := s.eventSink
		s.mu.Unlock()
		if sink == nil {
			return
		}
		if start {
			sink <- Event{Type: EventNotice, Notice: "context is near the limit, compacting history…"}
			return
		}
		switch v := info.(type) {
		case *CompactInfo:
			if v != nil {
				sink <- Event{Type: EventNotice, Notice: fmt.Sprintf("context compacted: %d messages summarized", v.RemovedMessages)}
				return
			}
		case map[string]any:
			if errText, _ := v["error"].(string); errText != "" {
				sink <- Event{Type: EventNotice, Notice: "compaction failed: " + errText}
				return
			}
		}
		sink <- Event{Type: EventNotice, Notice: "context compacted"}
	}
}

func (s *Service) RecordUsage(u *ai.Usage) {
	if u == nil {
		return
	}
	s.mu.Lock()
	s.lastUsage = u
	s.totalUsage.InputTokens += u.InputTokens
	s.totalUsage.OutputTokens += u.OutputTokens
	s.totalUsage.CacheRead += u.CacheRead
	s.totalUsage.CacheWrite += u.CacheWrite
	if u.Cost != nil {
		cur := 0.0
		if s.totalUsage.Cost != nil {
			cur = *s.totalUsage.Cost
		}
		sum := cur + *u.Cost
		s.totalUsage.Cost = &sum
	}
	s.mu.Unlock()
}

func (s *Service) IsStreaming() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streaming
}

func (s *Service) SetSteeringMode(mode string) {
	s.mu.Lock()
	s.steeringMode = mode
	s.mu.Unlock()
}

func (s *Service) SetFollowUpMode(mode string) {
	s.mu.Lock()
	s.followUpMode = mode
	s.mu.Unlock()
}

func (s *Service) SetModel(mc config.ModelConfig) {
	s.mu.Lock()
	s.Model = mc
	s.mu.Unlock()
}

func (s *Service) ActiveModelConfig() config.ModelConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Model
}

// Prompt начинает обработку сообщения пользователя. Ошибка, если уже идёт стрим без steer/followup.
func (s *Service) Prompt(parent context.Context, message string, streamingBehavior string, events chan<- Event) error {
	s.mu.Lock()
	if s.streaming {
		switch streamingBehavior {
		case "steer":
			images := s.takePendingImagesLocked()
			msg := message
			if len(s.pendingAttachments) > 0 {
				expanded, err := ExpandAttachments(s.Config.Workspace, msg, s.pendingAttachments)
				if err != nil {
					s.pendingAttachments = nil
					s.pendingImages = nil
					s.mu.Unlock()
					close(events)
					return err
				}
				msg = expanded
				s.pendingAttachments = nil
			}
			msg = PromptWithImages(msg, images)
			s.steerQueue = append(s.steerQueue, QueuedMessage{Text: msg, Images: images})
			s.mu.Unlock()
			close(events)
			return nil
		case "followUp":
			images := s.takePendingImagesLocked()
			msg := message
			if len(s.pendingAttachments) > 0 {
				expanded, err := ExpandAttachments(s.Config.Workspace, msg, s.pendingAttachments)
				if err != nil {
					s.pendingAttachments = nil
					s.pendingImages = nil
					s.mu.Unlock()
					close(events)
					return err
				}
				msg = expanded
				s.pendingAttachments = nil
			}
			msg = PromptWithImages(msg, images)
			s.followUpQueue = append(s.followUpQueue, QueuedMessage{Text: msg, Images: images})
			s.mu.Unlock()
			close(events)
			return nil
		default:
			s.mu.Unlock()
			close(events)
			return ErrStreaming
		}
	}
	s.streaming = true
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.eventSink = events
	pendingAttachments := append([]string(nil), s.pendingAttachments...)
	pendingImages := append([]ai.ImageContent(nil), s.pendingImages...)
	s.pendingAttachments = nil
	s.pendingImages = nil
	s.mu.Unlock()

	clean := s.PrepareMessage(message)
	if expanded, err := ExpandAttachments(s.Config.Workspace, clean, pendingAttachments); err != nil {
		s.mu.Lock()
		s.streaming = false
		s.cancel = nil
		s.eventSink = nil
		s.mu.Unlock()
		close(events)
		return err
	} else {
		clean = expanded
	}
	clean = PromptWithImages(clean, pendingImages)
	s.runWg.Add(1)
	go s.runSession(ctx, clean, pendingImages, events)
	return nil
}

// ContinueRun возобновляет цикл агента без нового user-промпта (RunContinue).
func (s *Service) ContinueRun(parent context.Context, events chan<- Event) error {
	s.mu.Lock()
	if s.streaming {
		s.mu.Unlock()
		close(events)
		return ErrStreaming
	}
	s.streaming = true
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.eventSink = events
	s.mu.Unlock()

	s.runWg.Add(1)
	go s.runSessionContinue(ctx, events)
	return nil
}

// SetStopAfterTurn вооружает ShouldStopAfterTurn на следующий завершённый tool-ход
// (shouldStopAfterTurn). Срабатывает один раз при вызове хука.
func (s *Service) SetStopAfterTurn(v bool) {
	s.mu.Lock()
	s.stopAfterTurn = v
	s.mu.Unlock()
}

func (s *Service) takeStopAfterTurn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.stopAfterTurn
	s.stopAfterTurn = false
	return v
}

func (s *Service) takePendingImagesLocked() []ai.ImageContent {
	images := append([]ai.ImageContent(nil), s.pendingImages...)
	s.pendingImages = nil
	return images
}

func (s *Service) Steer(message string, images []ai.ImageContent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.streaming {
		return ErrNotStreaming
	}
	if len(images) == 0 {
		images = append([]ai.ImageContent(nil), s.pendingImages...)
		s.pendingImages = nil
	}
	s.steerQueue = append(s.steerQueue, QueuedMessage{Text: message, Images: images})
	return nil
}

func (s *Service) FollowUp(message string, images []ai.ImageContent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.streaming {
		return ErrNotStreaming
	}
	if len(images) == 0 {
		images = append([]ai.ImageContent(nil), s.pendingImages...)
		s.pendingImages = nil
	}
	s.followUpQueue = append(s.followUpQueue, QueuedMessage{Text: message, Images: images})
	return nil
}

func (s *Service) Abort() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// DrainQueues возвращает и очищает все ожидающие steer/follow-up сообщения (восстановление после Esc abort).
func (s *Service) DrainQueues() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, q := range s.steerQueue {
		out = append(out, q.Text)
	}
	for _, q := range s.followUpQueue {
		out = append(out, q.Text)
	}
	s.steerQueue = nil
	s.followUpQueue = nil
	return out
}

func (s *Service) waitForRun(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		s.runWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// AbortAndWait отменяет активный run и ждёт его завершения.
func (s *Service) AbortAndWait(ctx context.Context) bool {
	wasStreaming := s.IsStreaming()
	s.Abort()
	if wasStreaming {
		s.waitForRun(ctx)
	}
	return wasStreaming
}

func (s *Service) QueueSnapshot() (steering, followUp []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, q := range s.steerQueue {
		steering = append(steering, q.Text)
	}
	for _, q := range s.followUpQueue {
		followUp = append(followUp, q.Text)
	}
	return steering, followUp
}

func (s *Service) takeFollowUp() (string, []ai.ImageContent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.followUpQueue) == 0 {
		return "", nil
	}
	if s.followUpMode == "one-at-a-time" {
		q := s.followUpQueue[0]
		s.followUpQueue = s.followUpQueue[1:]
		return s.PrepareMessage(q.Text), q.Images
	}
	var texts []string
	var imgs []ai.ImageContent
	for _, q := range s.followUpQueue {
		texts = append(texts, s.PrepareMessage(q.Text))
		imgs = append(imgs, q.Images...)
	}
	s.followUpQueue = nil
	return strings.Join(texts, "\n"), imgs
}

func (s *Service) takeSteer() (string, []ai.ImageContent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.steerQueue) == 0 {
		return "", nil
	}
	if s.steeringMode == "one-at-a-time" {
		q := s.steerQueue[0]
		s.steerQueue = s.steerQueue[1:]
		return s.PrepareMessage(q.Text), q.Images
	}
	var texts []string
	var imgs []ai.ImageContent
	for _, q := range s.steerQueue {
		texts = append(texts, s.PrepareMessage(q.Text))
		imgs = append(imgs, q.Images...)
	}
	s.steerQueue = nil
	return strings.Join(texts, "\n"), imgs
}

func (s *Service) PendingSteer() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.steerQueue) > 0
}
