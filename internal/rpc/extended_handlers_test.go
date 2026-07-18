package rpc

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/agent"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-ai/provider"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-agent/tools"

	_ "github.com/stelmakhdigital/stell-ai/provider/mock"
)

func TestGetSessionStatsRPC(t *testing.T) {
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
	inBuf.WriteString(`{"type":"get_session_stats","id":"1"}`)
	inBuf.WriteString("\n")

	srv := &Server{In: &inBuf, Out: &outBuf, ErrOut: &outBuf, Svc: svc, Cfg: cfg, Models: cfg.Models}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.respEnc = json.NewEncoder(srv.Out)

	if err := srv.Serve(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(outBuf.Bytes(), []byte(`"command":"get_session_stats"`)) {
		t.Fatalf("missing response: %s", outBuf.String())
	}
}

func TestCycleThinkingLevelRPC(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Settings: config.DefaultSettings(), GlobalDir: dir, Workspace: dir}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	sess := session.NewManager(dir)
	svc := agent.NewService(cfg, reg, rt, sess, "", config.ModelConfig{Name: "m"}, nil, nil)

	var inBuf, outBuf bytes.Buffer
	inBuf.WriteString(`{"type":"cycle_thinking_level","id":"2"}`)
	inBuf.WriteString("\n")
	srv := &Server{In: &inBuf, Out: &outBuf, ErrOut: &outBuf, Svc: svc, Cfg: cfg}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.respEnc = json.NewEncoder(srv.Out)
	_ = srv.Serve(t.Context())
	if !bytes.Contains(outBuf.Bytes(), []byte("thinkingLevel")) {
		t.Fatalf("missing thinking level: %s", outBuf.String())
	}
}
