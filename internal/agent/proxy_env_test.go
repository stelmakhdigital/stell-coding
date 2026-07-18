package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stelmakhdigital/stell-ai"
	"github.com/stelmakhdigital/stell-coding/internal/config"
)

func TestStreamFnFromEnvNilWhenUnset(t *testing.T) {
	t.Setenv("STELL_PROXY_URL", "")
	if StreamFnFromEnv() != nil {
		t.Fatal("expected nil")
	}
}

func TestStreamFnFromEnvProxyChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/stream" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"text_delta\",\"delta\":\"via-proxy\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"done\",\"reason\":\"stop\"}\n\n")
	}))
	defer srv.Close()

	t.Setenv("STELL_PROXY_URL", srv.URL)
	t.Setenv("STELL_PROXY_TOKEN", "tok")
	fn := StreamFnFromEnv()
	if fn == nil {
		t.Fatal("nil StreamFn")
	}
	ch, err := fn(context.Background(), ai.ChatRequest{Model: "m", Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	var tokens string
	for ev := range ch {
		if ev.Type == ai.EventToken {
			tokens += ev.Token
		}
		if ev.Type == ai.EventError {
			t.Fatal(ev.Err)
		}
	}
	if tokens != "via-proxy" {
		t.Fatalf("tokens=%q", tokens)
	}
}

func TestNewServicePicksUpProxyEnv(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"text_delta\",\"delta\":\"svc\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"done\",\"reason\":\"stop\"}\n\n")
	}))
	defer srv.Close()
	t.Setenv("STELL_PROXY_URL", srv.URL)
	t.Setenv("STELL_PROXY_TOKEN", "t")

	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	svc := NewService(cfg, nil, nil, nil, "", cfg.Models[0], nil, nil)
	if svc.StreamFn == nil {
		t.Fatal("Service.StreamFn nil")
	}
	ch, err := svc.StreamFn(context.Background(), ai.ChatRequest{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	var tok string
	for ev := range ch {
		if ev.Type == ai.EventToken {
			tok += ev.Token
		}
	}
	if tok != "svc" {
		t.Fatalf("tok=%q", tok)
	}
}
