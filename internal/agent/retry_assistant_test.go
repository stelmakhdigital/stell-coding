package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
	"github.com/stelmakhdigital/ai/provider"
	"github.com/stelmakhdigital/ai/provider/mock"
	"stell/agent/session"
	"stell/agent/tools"

	_ "github.com/stelmakhdigital/ai/provider/mock"
)

func TestWaitAssistantRetryPayload(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		Workspace: dir,
		GlobalDir: dir,
	}
	cfg.Settings.Retry.BaseDelayMs = 1
	svc := NewService(cfg, nil, nil, session.NewManager(dir), "", cfg.Models[0], nil, nil)
	svc.SetAutoRetry(true)

	if _, err := svc.Sessions.AppendMessage(ai.Message{Role: ai.RoleUser, Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	id, err := svc.Sessions.BeginAssistantMessage()
	if err != nil {
		t.Fatal(err)
	}
	msg := ai.AssistantMessage(nil, nil, "error", "429 rate limit exceeded")
	if err := svc.Sessions.PatchAssistantMessage(id, msg); err != nil {
		t.Fatal(err)
	}

	events := make(chan Event, 8)
	go func() {
		if !svc.waitAssistantRetry(context.Background(), events, 1) {
			t.Error("waitAssistantRetry returned false")
		}
		close(events)
	}()

	var start, end Event
	for ev := range events {
		switch ev.Type {
		case EventAutoRetryStart:
			start = ev
		case EventAutoRetryEnd:
			end = ev
		}
	}
	if start.AutoRetry == nil || start.AutoRetry.Attempt != 1 {
		t.Fatalf("start=%+v", start.AutoRetry)
	}
	if !strings.Contains(start.AutoRetry.ErrorMessage, "429") {
		t.Fatalf("errorMessage=%q", start.AutoRetry.ErrorMessage)
	}
	if end.AutoRetry == nil || !end.AutoRetry.Success {
		t.Fatalf("end=%+v", end.AutoRetry)
	}
}

func TestAgentEndRetryPayload(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		Workspace: dir,
		GlobalDir: dir,
	}
	cfg.Settings.Retry.Enabled = boolPtr(true)
	cfg.Settings.Retry.MaxRetries = 0
	cfg.Settings.Retry.BaseDelayMs = 1

	fail := []ai.ChatEvent{
		mock.Token("partial"),
		mock.Err(errors.New("429 rate limit exceeded")),
	}
	okScript := []ai.ChatEvent{mock.Token("recovered"), mock.Done(1, 1)}
	mp := mock.New("mock", fail, okScript)
	reg := provider.NewRegistry()
	reg.Register(cfg.Models[0], mp)
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	if err := tools.RegisterBuiltins(rt); err != nil {
		t.Fatal(err)
	}
	svc := NewService(cfg, reg, rt, session.NewManager(dir), "", cfg.Models[0], nil, nil)
	svc.SetAutoRetry(true)

	events := make(chan Event, 64)
	if err := svc.Prompt(context.Background(), "hi", "", events); err != nil {
		t.Fatal(err)
	}

	var retryStart *AutoRetryInfo
	var agentEnds []Event
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				goto done
			}
			switch ev.Type {
			case EventAutoRetryStart:
				cp := *ev.AutoRetry
				retryStart = &cp
			case EventDone:
				agentEnds = append(agentEnds, ev)
			}
		case <-deadline:
			t.Fatal("timeout waiting for events")
		}
	}
done:
	if retryStart == nil {
		t.Fatal("missing auto_retry_start payload")
	}
	if retryStart.Attempt != 1 {
		t.Fatalf("attempt=%d want 1", retryStart.Attempt)
	}
	if len(agentEnds) < 2 {
		t.Fatalf("agent_end count=%d want >=2", len(agentEnds))
	}
	if !agentEnds[0].WillRetry {
		t.Fatalf("first agent_end willRetry=%v want true", agentEnds[0].WillRetry)
	}
	if agentEnds[len(agentEnds)-1].StopReason != "completed" {
		t.Fatalf("final stopReason=%q want completed", agentEnds[len(agentEnds)-1].StopReason)
	}
}

func TestShouldRetryAfterStreamError(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	cfg.Settings.Retry.MaxRetries = 0
	fail := []ai.ChatEvent{mock.Token("x"), mock.Err(errors.New("429 rate limit exceeded"))}
	mp := mock.New("mock", fail)
	reg := provider.NewRegistry()
	reg.Register(cfg.Models[0], mp)
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	_ = tools.RegisterBuiltins(rt)
	ag := &Agent{
		Config: cfg, Registry: reg, Tools: rt,
		Sessions: session.NewManager(dir), Model: cfg.Models[0],
	}
	events := make(chan Event, 64)
	go ag.Run(context.Background(), "hi", events)
	for range events {
	}
	msg, ok := ag.Sessions.BuildMessages()[len(ag.Sessions.BuildMessages())-1], false
	for _, m := range ag.Sessions.BuildMessages() {
		if m.Role == ai.RoleAssistant {
			msg = m
			ok = true
		}
	}
	if !ok {
		t.Fatal("no assistant")
	}
	if msg.StopReason != "error" {
		t.Fatalf("stopReason=%q", msg.StopReason)
	}
	if msg.ErrorMessage == "" {
		t.Fatal("no error message")
	}
	svc := NewService(cfg, reg, rt, ag.Sessions, "", cfg.Models[0], nil, nil)
	svc.SetAutoRetry(true)
	if !svc.shouldRetryAssistantError() {
		t.Fatalf("should retry: err=%q reason=%q", msg.ErrorMessage, msg.StopReason)
	}
}
