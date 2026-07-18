package extensions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stelmakhdigital/stell-agent/tools"
	"github.com/stelmakhdigital/stell-coding/internal/config"
)

type stubHost struct {
	msg string
}

func (h *stubHost) ExtensionSendUserMessage(ctx context.Context, message, deliverAs string) error {
	h.msg = message
	return nil
}

func (h *stubHost) ExtensionSendMessage(customType, text string, data json.RawMessage) (string, error) {
	return "entry-custom", nil
}

func (h *stubHost) ExtensionReload(ctx context.Context) ([]ReloadStatus, error) {
	return nil, nil
}

func (h *stubHost) ExtensionAppendEntry(text string) (string, error) {
	return "entry-1", nil
}

func (h *stubHost) ExtensionAppendTypedEntry(customType, text string, data json.RawMessage, asMessage bool) (string, error) {
	return "entry-typed", nil
}

func (h *stubHost) ExtensionSetModel(name string) error { return nil }

func (h *stubHost) ExtensionGetThinkingLevel() string { return "off" }

func (h *stubHost) ExtensionSetThinkingLevel(level string) {}

func (h *stubHost) ExtensionSetLabel(label string) error { return nil }

func (h *stubHost) ExtensionRegisterProvider(name string, cfg ProviderOverrideConfig, owner string) error {
	return nil
}

func (h *stubHost) ExtensionUnregisterProvider(name string) error { return nil }

func (h *stubHost) ExtensionListThemes() []map[string]string { return nil }

func TestHandleHostRegisterTools(t *testing.T) {
	rt := tools.NewRuntime(tools.Env{Workspace: t.TempDir()})
	s := NewSupervisor(t.TempDir(), nil, rt)
	owner := &runningExt{toolNames: nil, client: &ProcessClient{}}
	defs := []ToolDef{{Name: "dyn", Description: "dyn", Schema: map[string]any{"type": "object"}}}
	if err := s.registerTools(owner, defs); err != nil {
		t.Fatal(err)
	}
	if len(rt.Defs()) == 0 {
		t.Fatal("expected tool registered")
	}
}

func TestHandleHostSendUserMessage(t *testing.T) {
	host := &stubHost{}
	s := &Supervisor{Host: host}
	raw := []byte(`{"message":"hi","deliverAs":"steer"}`)
	_, err := s.HandleHostRequest(context.Background(), nil, nil, "host/agent/send_user_message", raw)
	if err != nil {
		t.Fatal(err)
	}
	if host.msg != "hi" {
		t.Fatalf("msg=%q", host.msg)
	}
}

func TestHandleHostSendMessage(t *testing.T) {
	host := &stubHost{}
	s := &Supervisor{Host: host}
	raw := []byte(`{"message":"ctx note","customType":"note"}`)
	res, err := s.HandleHostRequest(context.Background(), nil, nil, "host/agent/send_message", raw)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := res.(map[string]any)
	if !ok || m["entryId"] != "entry-custom" {
		t.Fatalf("res=%v", res)
	}
}

func TestHandleHostSetActiveTools(t *testing.T) {
	rt := tools.NewRuntime(tools.Env{Workspace: t.TempDir()})
	if err := tools.RegisterBuiltins(rt); err != nil {
		t.Fatal(err)
	}
	s := &Supervisor{Runtime: rt}
	raw := []byte(`{"tools":["read"]}`)
	_, err := s.HandleHostRequest(context.Background(), nil, nil, "host/tools/set_active", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.Defs()) != 1 || rt.Defs()[0].Name != "read" {
		t.Fatalf("defs=%v", rt.Defs())
	}
}

func TestHandleHostUIInput(t *testing.T) {
	var proto *UIProtocol
	proto = NewUIProtocol(func(req UIRequest) {
		if got, _ := req.Data["value"].(string); got != "hello" {
			t.Errorf("value=%q", got)
		}
		proto.Respond(req.ID, map[string]any{"value": "edited"})
	})
	s := &Supervisor{UIHost: &UIHost{UI: proto}}
	raw := []byte(`{"message":"Edit transcript","value":"hello"}`)
	res, err := s.HandleHostRequest(context.Background(), nil, nil, "host/ui/input", raw)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("res=%v", res)
	}
	if m["value"] != "edited" {
		t.Fatalf("value=%v", m["value"])
	}
	if cancelled, _ := m["cancelled"].(bool); cancelled {
		t.Fatal("expected not cancelled")
	}
}

func TestHandleHostGetFlag(t *testing.T) {
	s := NewSupervisor(t.TempDir(), nil, nil)
	owner := &runningExt{extName: "demo", client: &ProcessClient{}}
	s.registerFlag(owner, "verbose", "verbose logs", "boolean", true)
	raw := []byte(`{"name":"verbose"}`)
	res, err := s.HandleHostRequest(context.Background(), nil, owner, "host/flags/get", raw)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := res.(map[string]any)
	if !ok || m["value"] != true {
		t.Fatalf("res=%v", res)
	}
}

func TestProviderOverridesApplyBaseURL(t *testing.T) {
	base := []config.ModelConfig{{Name: "m1", Provider: "anthropic", Model: "claude", APIBase: "https://api.anthropic.com"}}
	po := NewProviderOverrides(base)
	if err := po.Register("anthropic", ProviderOverrideConfig{BaseURL: "http://localhost:8080"}, "ext/demo"); err != nil {
		t.Fatal(err)
	}
	models := po.Models()
	if models[0].APIBase != "http://localhost:8080" {
		t.Fatalf("base=%q", models[0].APIBase)
	}
	po.Unregister("anthropic")
	models = po.Models()
	if models[0].APIBase != "https://api.anthropic.com" {
		t.Fatalf("restored base=%q", models[0].APIBase)
	}
}

func TestRendererRegistry(t *testing.T) {
	r := NewRendererRegistry()
	r.RegisterEntryRenderer("badge", "Badge")
	got := r.FormatEntry("badge", "hello", nil)
	if got != "[Badge] hello" {
		t.Fatalf("got=%q", got)
	}
}
