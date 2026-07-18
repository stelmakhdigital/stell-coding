package config

import "testing"

func TestResolveChatTemplateKwargs(t *testing.T) {
	raw := map[string]any{
		"thinking": map[string]any{"$var": "thinking.enabled"},
		"effort":   map[string]any{"$var": "thinking.effort"},
	}
	out := ResolveChatTemplateKwargs(raw, "low", nil)
	if out["thinking"] != true {
		t.Fatalf("thinking=%v", out["thinking"])
	}
	if out["effort"] != "low" {
		t.Fatalf("effort=%v", out["effort"])
	}
}
