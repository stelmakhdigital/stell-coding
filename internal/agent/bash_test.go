package agent

import (
	"context"
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-ai/provider"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-agent/tools"
)

func TestRunBashDefersDuringStreaming(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Settings: config.DefaultSettings(), Workspace: dir}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir, Trusted: true})
	sess := session.NewManager(dir)
	svc := NewService(cfg, reg, rt, sess, "", config.ModelConfig{}, nil, nil)

	svc.mu.Lock()
	svc.streaming = true
	svc.mu.Unlock()

	res, err := svc.RunBash(context.Background(), "echo hi", RunBashOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output == "" {
		t.Fatal("expected output")
	}
	if len(sess.Entries) != 0 {
		t.Fatalf("expected deferred bash, got %d entries", len(sess.Entries))
	}
	svc.FlushPendingBash()
	if len(sess.Entries) != 1 || sess.Entries[0].Type != "message" {
		t.Fatalf("expected bash message entry after flush, got %+v", sess.Entries)
	}
	if sess.Entries[0].Message == nil || sess.Entries[0].Message.Role != "bashExecution" {
		t.Fatalf("expected bashExecution role, got %+v", sess.Entries[0].Message)
	}
}

func TestRunBashRejectsConcurrent(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Settings: config.DefaultSettings(), Workspace: dir}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir, Trusted: true})
	sess := session.NewManager(dir)
	svc := NewService(cfg, reg, rt, sess, "", config.ModelConfig{}, nil, nil)

	svc.bashMu.Lock()
	svc.bashCancel = func() {}
	svc.bashMu.Unlock()

	_, err := svc.RunBash(context.Background(), "echo hi", RunBashOptions{})
	if err != ErrBashRunning {
		t.Fatalf("got %v, want ErrBashRunning", err)
	}
}
