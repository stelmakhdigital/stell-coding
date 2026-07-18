package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/stelmakhdigital/stell-coding/internal/agent"
	"github.com/stelmakhdigital/stell-ai"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-coding/internal/extensions"
)

// Server — JSONL RPC-режим (команды stdin, ответы + события stdout).
type Server struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
	Svc         *agent.Service
	Cfg         *config.Config
	Models      []config.ModelConfig
	GrantBroker *extensions.GrantBroker

	mu       sync.Mutex
	eventEnc *json.Encoder
	respEnc  *json.Encoder
	runWg    sync.WaitGroup
}

func NewServer(svc *agent.Service, cfg *config.Config) *Server {
	s := &Server{
		In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr,
		Svc: svc, Cfg: cfg, Models: cfg.Models,
	}
	s.eventEnc = json.NewEncoder(s.Out)
	s.respEnc = json.NewEncoder(s.Out)
	return s
}

func (s *Server) Serve(ctx context.Context) error {
	s.eventEnc.SetEscapeHTML(false)
	s.respEnc.SetEscapeHTML(false)

	sc := bufio.NewScanner(s.In)
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	for sc.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var head struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &head); err != nil {
			s.respond("", "", false, nil, err.Error())
			continue
		}
		if head.Type == "" {
			s.respond(head.ID, "", false, nil, "missing type")
			continue
		}
		s.handle(ctx, head.ID, head.Type, json.RawMessage(line))
	}
	return sc.Err()
}

func (s *Server) handle(ctx context.Context, id, typ string, raw json.RawMessage) {
	if s.handleSessionCommands(ctx, id, typ, raw) {
		return
	}
	if s.handleExtendedCommands(ctx, id, typ, raw) {
		return
	}
	switch typ {
	case "prompt":
		var p struct {
			Message           string            `json:"message"`
			StreamingBehavior string            `json:"streamingBehavior"`
			Images            []ai.ImageContent `json:"images"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, "prompt", false, nil, err.Error())
			return
		}
		if len(p.Images) > 0 {
			s.Svc.SetPendingImages(p.Images)
		}
		behavior := p.StreamingBehavior
		if behavior == "" && s.Svc.IsStreaming() {
			s.respond(id, "prompt", false, nil, "agent is streaming; set streamingBehavior to steer or followUp")
			return
		}
		events := make(chan agent.Event, 64)
		if err := s.Svc.Prompt(ctx, p.Message, behavior, events); err != nil {
			s.respond(id, "prompt", false, nil, err.Error())
			return
		}
		s.runWg.Add(1)
		go func() {
			defer s.runWg.Done()
			s.streamEvents(events)
		}()
		s.respond(id, "prompt", true, nil, "")

	case "steer":
		var p struct {
			Message string            `json:"message"`
			Images  []ai.ImageContent `json:"images"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, "steer", false, nil, err.Error())
			return
		}
		if err := s.Svc.Steer(p.Message, p.Images); err != nil {
			s.respond(id, "steer", false, nil, err.Error())
			return
		}
		s.emitQueueUpdate()
		s.respond(id, "steer", true, nil, "")

	case "follow_up":
		var p struct {
			Message string            `json:"message"`
			Images  []ai.ImageContent `json:"images"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, "follow_up", false, nil, err.Error())
			return
		}
		if err := s.Svc.FollowUp(p.Message, p.Images); err != nil {
			s.respond(id, "follow_up", false, nil, err.Error())
			return
		}
		s.emitQueueUpdate()
		s.respond(id, "follow_up", true, nil, "")

	case "abort":
		s.Svc.Abort()
		s.respond(id, "abort", true, nil, "")

	case "continue_run", "agent_continue":
		events := make(chan agent.Event, 64)
		if err := s.Svc.ContinueRun(ctx, events); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return
		}
		s.runWg.Add(1)
		go func() {
			defer s.runWg.Done()
			s.streamEvents(events)
		}()
		s.respond(id, typ, true, nil, "")

	case "set_stop_after_turn":
		var p struct {
			Stop bool `json:"stop"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, "set_stop_after_turn", false, nil, err.Error())
			return
		}
		s.Svc.SetStopAfterTurn(p.Stop)
		s.respond(id, "set_stop_after_turn", true, map[string]any{"stop": p.Stop}, "")

	case "get_state":
		st := s.Svc.GetState()
		s.respond(id, "get_state", true, st, "")

	case "get_messages":
		msgs := s.Svc.Sessions.BuildMessages()
		s.respond(id, "get_messages", true, map[string]any{"messages": msgs}, "")

	case "get_available_models":
		s.respond(id, "get_available_models", true, map[string]any{"models": s.Svc.AvailableModels()}, "")

	case "get_skills":
		var skills any = []any{}
		if s.Svc.Catalog != nil && s.Svc.Catalog.Skills != nil {
			skills = s.Svc.Catalog.Skills.List()
		}
		s.respond(id, "get_skills", true, map[string]any{"skills": skills}, "")

	case "get_prompts":
		var prompts any = []any{}
		if s.Svc.Catalog != nil && s.Svc.Catalog.Prompts != nil {
			prompts = s.Svc.Catalog.Prompts.List()
		}
		s.respond(id, "get_prompts", true, map[string]any{"prompts": prompts}, "")

	case "run_prompt":
		var p struct {
			Name string   `json:"name"`
			Args []string `json:"args"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, "run_prompt", false, nil, err.Error())
			return
		}
		text, err := s.Svc.RenderPrompt(p.Name, p.Args)
		if err != nil {
			s.respond(id, "run_prompt", false, nil, err.Error())
			return
		}
		s.respond(id, "run_prompt", true, map[string]any{"text": text}, "")

	case "set_model":
		var p struct {
			ModelID string `json:"modelId"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, "set_model", false, nil, err.Error())
			return
		}
		if p.ModelID == "" {
			s.respond(id, "set_model", false, nil, "modelId required")
			return
		}
		found := false
		for _, m := range s.Models {
			if m.Name == p.ModelID || m.Model == p.ModelID {
				s.Svc.SetModelRecord(m)
				found = true
				break
			}
		}
		if !found {
			s.respond(id, "set_model", false, nil, fmt.Sprintf("model %q not found", p.ModelID))
			return
		}
		s.respond(id, "set_model", true, nil, "")

	case "set_steering_mode":
		var p struct {
			Mode string `json:"mode"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, "set_steering_mode", false, nil, err.Error())
			return
		}
		s.Svc.SetSteeringMode(p.Mode)
		s.respond(id, "set_steering_mode", true, nil, "")

	case "set_follow_up_mode":
		var p struct {
			Mode string `json:"mode"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, "set_follow_up_mode", false, nil, err.Error())
			return
		}
		s.Svc.SetFollowUpMode(p.Mode)
		s.respond(id, "set_follow_up_mode", true, nil, "")

	case "new_session":
		var p struct {
			ParentSession   string `json:"parentSession"`
			ParentSessionID string `json:"parentSessionId"`
		}
		_ = json.Unmarshal(raw, &p)
		parent := p.ParentSessionID
		if parent == "" {
			parent = p.ParentSession
		}
		cancelled, err := s.Svc.NewSession(ctx, parent)
		if err != nil {
			s.respond(id, "new_session", false, nil, err.Error())
			return
		}
		s.respond(id, "new_session", true, map[string]any{
			"cancelled":         cancelled,
			"sessionId":         s.Svc.Sessions.Header.ID,
			"parentSessionId":   s.Svc.Sessions.Header.ParentSessionID,
		}, "")

	case "reload":
		st, err := s.Svc.ReloadExtensions(ctx)
		if err != nil {
			s.respond(id, "reload", false, nil, err.Error())
			return
		}
		s.respond(id, "reload", true, map[string]any{"extensions": st}, "")

	case "get_commands":
		cmds := s.aggregateCommands()
		s.respond(id, "get_commands", true, map[string]any{"commands": cmds}, "")

	default:
		s.respond(id, typ, false, nil, "unknown command")
	}
}

func (s *Server) streamEvents(events <-chan agent.Event) {
	s.emit(map[string]any{"type": "agent_start"})
	emitTurnEvents(s, true)
	mapper := NewEventMapper()
	for ev := range events {
		if ev.Type == agent.EventDone {
			for _, obj := range mapper.Map(ev) {
				s.emit(obj)
			}
			emitTurnEvents(s, false)
			continue
		}
		for _, obj := range mapper.Map(ev) {
			s.emit(obj)
		}
	}
	s.emit(map[string]any{"type": "agent_settled"})
}

func (s *Server) emitQueueUpdate() {
	steer, follow := s.Svc.QueueSnapshot()
	s.emit(map[string]any{
		"type":     "queue_update",
		"steering": steer,
		"followUp": follow,
	})
}

func (s *Server) emit(obj map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.eventEnc.Encode(obj); err != nil {
		_, _ = fmt.Fprintf(s.errOut(), "stell rpc: encode event: %v\n", err)
	}
}

func (s *Server) respond(id, command string, success bool, data any, errMsg string) {
	resp := map[string]any{
		"type":    "response",
		"command": command,
		"success": success,
	}
	if id != "" {
		resp["id"] = id
	}
	if data != nil {
		resp["data"] = data
	}
	if !success && errMsg != "" {
		resp["error"] = errMsg
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.respEnc.Encode(resp); err != nil {
		_, _ = fmt.Fprintf(s.errOut(), "stell rpc: encode response: %v\n", err)
	}
}

func (s *Server) errOut() io.Writer {
	if s.ErrOut != nil {
		return s.ErrOut
	}
	return os.Stderr
}

func (s *Server) aggregateCommands() []map[string]any {
	var out []map[string]any
	for _, c := range s.Svc.ExtensionCommands() {
		out = append(out, map[string]any{
			"name": c.Name, "description": c.Description, "source": c.Source,
		})
	}
	if s.Svc.Catalog != nil && s.Svc.Catalog.Skills != nil {
		for _, sk := range s.Svc.Catalog.Skills.List() {
			out = append(out, map[string]any{
				"name": "/skill:" + sk.Name, "description": sk.Description, "source": "skill",
			})
		}
	}
	if s.Svc.Catalog != nil && s.Svc.Catalog.Prompts != nil {
		for _, pr := range s.Svc.Catalog.Prompts.List() {
			out = append(out, map[string]any{
				"name": "/" + pr.Name, "description": pr.Description, "source": "prompt",
			})
		}
	}
	return out
}
