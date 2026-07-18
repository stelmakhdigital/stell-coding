package config

import "testing"

func TestApplyProviderConfigOverrides(t *testing.T) {
	models := []ModelConfig{{
		Name: "claude", Provider: "anthropic", ProviderID: "anthropic", Model: "claude-sonnet-4",
	}}
	providers := map[string]ProviderConfig{
		"anthropic": {
			BaseURL: "https://proxy.example.com/v1",
			ModelOverrides: map[string]ProviderModelEntry{
				"claude-sonnet-4": {ID: "claude-sonnet-4", ContextWindow: 200000},
			},
		},
	}
	out, err := applyProviderConfig(models, providers)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].APIBase != "https://proxy.example.com/v1" {
		t.Fatalf("baseUrl override: %s", out[0].APIBase)
	}
	if out[0].ContextWindow != 200000 {
		t.Fatalf("context override: %d", out[0].ContextWindow)
	}
}

func TestSupportsImage(t *testing.T) {
	unknown := ModelConfig{}
	if unknown.SupportsImage() {
		t.Fatal("empty input should not assume image support")
	}
	textOnly := ModelConfig{Input: []string{"text"}}
	if textOnly.SupportsImage() {
		t.Fatal("text-only should not support image")
	}
	vision := ModelConfig{Input: []string{"text", "image"}}
	if !vision.SupportsImage() {
		t.Fatal("vision model should support image")
	}
}

func TestSupportedThinkingLevelsWithHoles(t *testing.T) {
	null := (*string)(nil)
	mc := ModelConfig{ThinkingLevelMap: map[string]*string{
		"off": null, "minimal": null, "low": null, "medium": null, "xhigh": null,
		"high": strPtr("high"), "max": strPtr("max"),
	}}
	levels := SupportedThinkingLevels(mc)
	if len(levels) != 2 || levels[0] != "high" || levels[1] != "max" {
		t.Fatalf("levels: %v", levels)
	}
}

func strPtr(s string) *string { return &s }
