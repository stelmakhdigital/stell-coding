package config

import (
	"testing"
)

func TestParseProvidersMinimal(t *testing.T) {
	raw := `{
  "providers": {
    "ollama": {
      "baseUrl": "http://localhost:11434/v1",
      "api": "openai-completions",
      "apiKey": "ollama",
      "compat": { "supportsDeveloperRole": false, "supportsReasoningEffort": false },
      "models": [{ "id": "llama3.2" }]
    }
  }
}`
	mf, err := ParseModelsJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Models) != 1 {
		t.Fatalf("models: got %d", len(mf.Models))
	}
	mc := mf.Models[0]
	if mc.Model != "llama3.2" || mc.Provider != "openai" {
		t.Fatalf("model: %+v", mc)
	}
	if mc.ProviderID != "ollama" || !mc.Local {
		t.Fatalf("provider meta: %+v", mc)
	}
	if mc.Compat.SupportsDeveloperRole == nil || *mc.Compat.SupportsDeveloperRole {
		t.Fatalf("compat developer: %+v", mc.Compat)
	}
}

func TestParseLegacyModels(t *testing.T) {
	raw := `{"models":[{"name":"mock","provider":"mock","model":"mock"}]}`
	mf, err := ParseModelsJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if mf.Format != "legacy" || len(mf.Models) != 1 || mf.Models[0].Name != "mock" {
		t.Fatalf("legacy: %+v", mf)
	}
}

func TestAddProviderModelRoundTrip(t *testing.T) {
	mf := ModelsFile{Format: "providers", Providers: map[string]ProviderConfig{}}
	f := false
	err := mf.AddProviderModel("ollama", ProviderModelEntry{ID: "qwen2.5"}, ProviderConfig{
		BaseURL: "http://localhost:11434/v1",
		API:     "openai-completions",
		APIKey:  "ollama",
		Compat:  CompatSettings{SupportsDeveloperRole: &f, SupportsReasoningEffort: &f},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := MarshalModelsFile(mf)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseModelsJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Models) != 1 || back.Models[0].Model != "qwen2.5" {
		t.Fatalf("roundtrip: %+v", back.Models)
	}
}

func TestThinkingBudgetSettings(t *testing.T) {
	s := Settings{ThinkingBudgets: map[string]int{"low": 2048}}
	if got := ThinkingBudget(s, "low"); got != 2048 {
		t.Fatalf("budget: %d", got)
	}
}

func TestMapThinkingLevelUnsupported(t *testing.T) {
	null := (*string)(nil)
	mc := ModelConfig{ThinkingLevelMap: map[string]*string{"off": null}}
	_, ok := MapThinkingLevel(mc, "off")
	if ok {
		t.Fatal("expected unsupported")
	}
}
