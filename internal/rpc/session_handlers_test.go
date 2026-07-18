package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"stell/coding-agent/internal/agent"
	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
	"github.com/stelmakhdigital/ai/provider"
	"stell/agent/session"
	"stell/agent/tools"

	_ "github.com/stelmakhdigital/ai/provider/mock"
)

func TestGetTreeRPC(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	_ = tools.RegisterBuiltins(rt)
	sess := session.NewManager(dir)
	svc := agent.NewService(cfg, reg, rt, sess, "", cfg.Models[0], nil, nil)

	var inBuf, outBuf bytes.Buffer
	inBuf.WriteString(`{"type":"get_tree","id":"t1"}`)
	inBuf.WriteString("\n")

	srv := &Server{In: &inBuf, Out: &outBuf, ErrOut: io.Discard, Svc: svc, Cfg: cfg, Models: cfg.Models}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.respEnc = json.NewEncoder(srv.Out)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Serve(ctx)

	var found bool
	for _, line := range bytes.Split(outBuf.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var resp map[string]any
		if json.Unmarshal(line, &resp) != nil {
			continue
		}
		if resp["type"] == "response" && resp["command"] == "get_tree" && resp["success"] == true {
			found = true
		}
	}
	if !found {
		t.Fatalf("get_tree response missing: %s", outBuf.String())
	}
}

func TestAppendCustomMessageRPC(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	sess := session.NewManager(dir)
	svc := agent.NewService(cfg, reg, rt, sess, "", cfg.Models[0], nil, nil)

	var inBuf, outBuf bytes.Buffer
	inBuf.WriteString(`{"type":"append_custom_message","id":"c1","text":"hello custom"}`)
	inBuf.WriteString("\n")
	srv := &Server{In: &inBuf, Out: &outBuf, ErrOut: io.Discard, Svc: svc, Cfg: cfg, Models: cfg.Models}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.respEnc = json.NewEncoder(srv.Out)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Serve(ctx)

	found := false
	for _, line := range bytes.Split(outBuf.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var resp map[string]any
		if json.Unmarshal(line, &resp) != nil {
			continue
		}
		if resp["type"] == "response" && resp["command"] == "append_custom_message" && resp["success"] == true {
			found = true
		}
	}
	if !found {
		t.Fatalf("append_custom_message response missing: %s", outBuf.String())
	}
	for _, e := range sess.Entries {
		if e.Type == "custom_message" {
			return
		}
	}
	t.Fatal("custom_message entry not written")
}

func TestNewSessionParentRPC(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	sess := session.NewManager(dir)
	parentID := sess.Header.ID
	svc := agent.NewService(cfg, reg, rt, sess, "", cfg.Models[0], nil, nil)

	var inBuf, outBuf bytes.Buffer
	inBuf.WriteString(`{"type":"new_session","id":"n1","parentSessionId":"`)
	inBuf.WriteString(parentID)
	inBuf.WriteString(`"}`)
	inBuf.WriteString("\n")
	srv := &Server{In: &inBuf, Out: &outBuf, ErrOut: io.Discard, Svc: svc, Cfg: cfg, Models: cfg.Models}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.respEnc = json.NewEncoder(srv.Out)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Serve(ctx)

	if svc.Sessions.Header.ParentSessionID != parentID {
		t.Fatalf("parentSessionId=%q want %q", svc.Sessions.Header.ParentSessionID, parentID)
	}
	if svc.Sessions.Header.ID == parentID {
		t.Fatal("expected new session id")
	}
}

func TestGetEntriesLeafRestore(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	sess := session.NewManager(dir)
	id1, _ := sess.AppendMessage(ai.Message{Role: ai.RoleUser, Content: "a"})
	id2, _ := sess.AppendMessage(ai.Message{Role: ai.RoleUser, Content: "b"})
	svc := agent.NewService(cfg, reg, rt, sess, "", cfg.Models[0], nil, nil)

	var inBuf, outBuf bytes.Buffer
	inBuf.WriteString(`{"type":"get_entries","id":"e1","leafId":"`)
	inBuf.WriteString(id1)
	inBuf.WriteString(`"}`)
	inBuf.WriteString("\n")
	srv := &Server{In: &inBuf, Out: &outBuf, ErrOut: io.Discard, Svc: svc, Cfg: cfg, Models: cfg.Models}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.respEnc = json.NewEncoder(srv.Out)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Serve(ctx)

	if svc.Sessions.LeafID() != id2 {
		t.Fatalf("leaf=%q want %q (restored after get_entries)", svc.Sessions.LeafID(), id2)
	}
}

func TestCycleModelRPC(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings: config.DefaultSettings(),
		Models: []config.ModelConfig{
			{Name: "a", Provider: "mock", Model: "mock"},
			{Name: "b", Provider: "mock", Model: "mock"},
		},
		GlobalDir: dir,
		Workspace: dir,
	}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	sess := session.NewManager(dir)
	svc := agent.NewService(cfg, reg, rt, sess, "", cfg.Models[0], nil, nil)

	var inBuf, outBuf bytes.Buffer
	inBuf.WriteString(`{"type":"cycle_model","id":"m1"}`)
	inBuf.WriteString("\n")
	srv := &Server{In: &inBuf, Out: &outBuf, ErrOut: io.Discard, Svc: svc, Cfg: cfg, Models: cfg.Models}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.respEnc = json.NewEncoder(srv.Out)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Serve(ctx)
	if svc.Model.Name != "b" {
		t.Fatalf("model=%s", svc.Model.Name)
	}
}
