package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
	"github.com/stelmakhdigital/ai/provider"
	"github.com/stelmakhdigital/ai/provider/mock"
	"stell/agent/session"

	_ "github.com/stelmakhdigital/ai/provider/mock"
)

type retryOnceProvider struct {
	calls int
}

func (p *retryOnceProvider) Name() string { return "retry-once" }

func (p *retryOnceProvider) Chat(ctx context.Context, _ ai.ChatRequest) (<-chan ai.ChatEvent, error) {
	p.calls++
	if p.calls == 1 {
		return nil, errors.New("503 temporarily unavailable")
	}
	ch := make(chan ai.ChatEvent, 2)
	ch <- mock.Token("ok")
	ch <- mock.Done(1, 1)
	close(ch)
	return ch, nil
}

func TestAbortRetryStopsRetryLoop(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings: config.DefaultSettings(),
		Models:   []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		Workspace: dir, GlobalDir: dir,
	}
	cfg.Settings.Retry.Enabled = boolPtr(true)
	cfg.Settings.Retry.MaxRetries = 5
	cfg.Settings.Retry.BaseDelayMs = 10

	prov := &retryOnceProvider{}
	reg := provider.NewRegistry()
	reg.Register(cfg.Models[0], prov)
	svc := NewService(cfg, reg, nil, session.NewManager(dir), "", cfg.Models[0], nil, nil)
	svc.SetAutoRetry(true)

	ag := &Agent{
		Config: cfg, Registry: reg, Sessions: session.NewManager(dir), Model: cfg.Models[0],
		RetryEnabled: svc.AutoRetryEnabled,
		RetrySettings: func() config.RetrySettings { return cfg.Settings.Retry },
		ShouldAbortRetry: func() bool {
			svc.AbortRetry()
			return svc.TakeAbortRetry()
		},
	}
	_, err := chatWithRetry(context.Background(), prov, ai.ChatRequest{}, ag.retryControl(), nil)
	if err == nil {
		t.Fatal("expected retry aborted error")
	}
}

func boolPtr(v bool) *bool { return &v }

func TestRetryEventuallySucceeds(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings: config.DefaultSettings(),
		Models:   []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		Workspace: dir, GlobalDir: dir,
	}
	cfg.Settings.Retry.Enabled = boolPtr(true)
	cfg.Settings.Retry.MaxRetries = 3
	cfg.Settings.Retry.BaseDelayMs = 1

	prov := &retryOnceProvider{}
	reg := provider.NewRegistry()
	reg.Register(cfg.Models[0], prov)

	stream, err := chatWithRetry(context.Background(), prov, ai.ChatRequest{}, retryControl{
		settings: cfg.Settings.Retry,
		enabled:  true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev, ok := <-stream:
			if !ok {
				if prov.calls < 2 {
					t.Fatalf("calls=%d, want >= 2", prov.calls)
				}
				return
			}
			if ev.Type == ai.EventError {
				t.Fatalf("unexpected stream error: %v", ev.Err)
			}
		case <-deadline:
			t.Fatal("timeout waiting for retry stream")
		}
	}
}
