package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"stell/coding-agent/internal/agent"
	"stell/coding-agent/internal/config"
	"github.com/stelmakhdigital/ai/provider"
	"stell/agent/session"
	"stell/agent/tools"
)

func TestRPCBashDeniedWithoutTrust(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	rt := tools.NewRuntime(tools.Env{Workspace: dir, BashDeny: true})
	svc := agent.NewService(cfg, provider.NewRegistry(), rt, session.NewManager(dir), "", cfg.Models[0], nil, nil)

	var inBuf, outBuf bytes.Buffer
	inBuf.WriteString(`{"type":"bash","id":"1","command":"echo hi"}`)
	inBuf.WriteString("\n")

	srv := &Server{In: &inBuf, Out: &outBuf, ErrOut: io.Discard, Svc: svc, Cfg: cfg}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.respEnc = json.NewEncoder(srv.Out)
	srv.eventEnc.SetEscapeHTML(false)
	srv.respEnc.SetEscapeHTML(false)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Serve(ctx); err != nil && err != context.DeadlineExceeded {
		t.Fatal(err)
	}
	if !strings.Contains(outBuf.String(), `"success":false`) {
		t.Fatalf("expected bash denied, got %s", outBuf.String())
	}
}

func TestExportHTMLRejectsOutsideWorkspace(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	svc := agent.NewService(cfg, provider.NewRegistry(), tools.NewRuntime(tools.Env{Workspace: dir}), session.NewManager(dir), "", cfg.Models[0], nil, nil)

	var inBuf, outBuf bytes.Buffer
	inBuf.WriteString(`{"type":"export_html","id":"1","outputPath":"/tmp/evil.html"}`)
	inBuf.WriteString("\n")

	srv := &Server{In: &inBuf, Out: &outBuf, ErrOut: io.Discard, Svc: svc, Cfg: cfg}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.respEnc = json.NewEncoder(srv.Out)
	srv.eventEnc.SetEscapeHTML(false)
	srv.respEnc.SetEscapeHTML(false)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Serve(ctx)
	if !strings.Contains(outBuf.String(), `"success":false`) {
		t.Fatalf("expected export denied, got %s", outBuf.String())
	}
}

func TestExportHTMLWritesInsideWorkspace(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	svc := agent.NewService(cfg, provider.NewRegistry(), tools.NewRuntime(tools.Env{Workspace: dir}), session.NewManager(dir), "", cfg.Models[0], nil, nil)

	var inBuf, outBuf bytes.Buffer
	inBuf.WriteString(`{"type":"export_html","id":"1","outputPath":"out.html"}`)
	inBuf.WriteString("\n")

	srv := &Server{In: &inBuf, Out: &outBuf, ErrOut: io.Discard, Svc: svc, Cfg: cfg}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.respEnc = json.NewEncoder(srv.Out)
	srv.eventEnc.SetEscapeHTML(false)
	srv.respEnc.SetEscapeHTML(false)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Serve(ctx)
	if !strings.Contains(outBuf.String(), `"success":true`) {
		t.Fatalf("expected export ok, got %s", outBuf.String())
	}
}
