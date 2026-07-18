package tui

import (
	"context"
	"testing"
	"time"

	"github.com/stelmakhdigital/stell-coding/internal/agent"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-coding/internal/extensions"
	"github.com/stelmakhdigital/stell-ai/provider"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-agent/tools"
)

// Ensure Init's long-lived wait cmds do not block the interactive loop setup.
func TestScheduleInitDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Workspace: dir,
		GlobalDir: dir,
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
	}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	_ = tools.RegisterBuiltins(rt)
	sess := session.NewManager(dir)
	svc := agent.NewService(cfg, reg, rt, sess, "", cfg.Models[0], nil, nil)

	grantCh := make(chan extensions.GrantRequest)
	uiCh := make(chan extensions.UIRequest)
	m := NewModel(context.Background(), Options{
		Service: svc,
		Config:  cfg,
		GrantCh: grantCh,
		UICh:    uiCh,
	})

	msgCh := make(chan Msg, 8)
	var schedule func(Cmd)
	schedule = func(cmd Cmd) {
		if cmd == nil {
			return
		}
		go func() {
			msg := cmd()
			if msg == nil {
				return
			}
			if bm, ok := msg.(batchMsg); ok {
				for _, c := range bm.cmds {
					schedule(c)
				}
				return
			}
			select {
			case msgCh <- msg:
			default:
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		schedule(m.Init())
		close(done)
	}()

	select {
	case <-done:
		// ok — Init returned immediately even though waitGrant/waitUI block
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Init schedule blocked on wait cmds")
	}
}
