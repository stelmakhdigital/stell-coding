package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
	"github.com/stelmakhdigital/ai/provider"
	"github.com/stelmakhdigital/ai/provider/mock"
	"stell/agent/session"
	"stell/agent/tools"

	_ "github.com/stelmakhdigital/ai/provider/mock"
)

type multimodalFailProvider struct {
	calls int
	inner ai.Provider
}

func (p *multimodalFailProvider) Name() string { return p.inner.Name() }

func (p *multimodalFailProvider) Chat(ctx context.Context, req ai.ChatRequest) (<-chan ai.ChatEvent, error) {
	p.calls++
	if p.calls == 1 {
		return nil, errors.New("image_url is not supported")
	}
	return p.inner.Chat(ctx, req)
}

func TestMultimodalStripAndRetry(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock", Input: []string{"text", "image"}}},
		GlobalDir: dir,
		Workspace: dir,
	}
	inner := mock.New("mock", []ai.ChatEvent{mock.Token("ok"), mock.Done(1, 1)})
	reg := provider.NewRegistry()
	reg.Register(cfg.Models[0], &multimodalFailProvider{inner: inner})
	sess := session.NewManager(dir)
	sess.AppendMessage(ai.Message{Role: ai.RoleUser, Content: "see this", Images: []ai.ImageContent{{Type: "image", Data: "x", MimeType: "image/png"}}})

	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	ag := &Agent{Config: cfg, Registry: reg, Tools: rt, Sessions: sess, Model: cfg.Models[0]}
	events := make(chan Event, 32)
	go ag.Run(context.Background(), "describe", events)

	var sawNotice bool
	for ev := range events {
		if ev.Type == EventNotice {
			sawNotice = true
		}
		if ev.Type == EventError {
			t.Fatalf("unexpected error: %v", ev.Err)
		}
	}
	if !sawNotice {
		t.Fatal("expected multimodal fallback notice")
	}
}
