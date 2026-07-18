package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
	"github.com/stelmakhdigital/ai/provider"
	"stell/agent/session"
	"stell/agent/tools"
)

func TestPromptSnapshotsAndClearsPending(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	_ = tools.RegisterBuiltins(rt)
	sess := session.NewManager(dir)
	svc := NewService(cfg, reg, rt, sess, "", cfg.Models[0], nil, nil)
	svc.SetPendingAttachments([]string{"note.txt"})

	events := make(chan Event, 64)
	if err := svc.Prompt(context.Background(), "hi", "", events); err != nil {
		t.Fatal(err)
	}

	svc.mu.Lock()
	files := len(svc.pendingAttachments)
	images := len(svc.pendingImages)
	svc.mu.Unlock()
	if files != 0 || images != 0 {
		t.Fatalf("expected pending cleared after Prompt snapshot, files=%d images=%d", files, images)
	}

	go func() {
		for range events {
		}
	}()
	svc.Abort()
}

func TestRecordUsageAccumulatesOnce(t *testing.T) {
	svc := &Service{}
	u := &ai.Usage{InputTokens: 10, OutputTokens: 5}
	svc.RecordUsage(u)
	svc.mu.Lock()
	total := svc.totalUsage.InputTokens
	svc.mu.Unlock()
	if total != 10 {
		t.Fatalf("expected 10 input tokens, got %d", total)
	}
}
