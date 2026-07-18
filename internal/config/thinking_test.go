package config

import "testing"

func TestAdjustMaxTokensForThinkingHigh(t *testing.T) {
	got := AdjustMaxTokensForThinking(16384, 16384, "high", nil)
	if got.MaxTokens != 16384 {
		t.Fatalf("maxTokens=%d want 16384", got.MaxTokens)
	}
	if got.ThinkingBudget != 15360 {
		t.Fatalf("thinkingBudget=%d want 15360", got.ThinkingBudget)
	}
}

func TestAdjustMaxTokensForThinkingMedium(t *testing.T) {
	got := AdjustMaxTokensForThinking(8192, 16384, "medium", nil)
	if got.MaxTokens != 16384 {
		t.Fatalf("maxTokens=%d want 16384", got.MaxTokens)
	}
	if got.ThinkingBudget != 8192 {
		t.Fatalf("thinkingBudget=%d want 8192", got.ThinkingBudget)
	}
}

func TestChatTokenBudgetOff(t *testing.T) {
	mc := ModelConfig{DefaultParams: InferenceParams{MaxTokens: 16384}}
	got, level := ChatTokenBudget(mc, Settings{}, "off")
	if level != "off" || got.ThinkingBudget != 0 || got.MaxTokens != 16384 {
		t.Fatalf("off budget: %+v level=%q", got, level)
	}
}

func TestModelMaxTokensDefault(t *testing.T) {
	if got := ModelMaxTokens(ModelConfig{}); got != DefaultMaxTokens {
		t.Fatalf("default=%d want %d", got, DefaultMaxTokens)
	}
}

func TestRequiresThinkingAsTextAlias(t *testing.T) {
	v := true
	c := CompatSettings{ReasoningAsContent: &v}
	if !c.RequiresThinkingAsTextEnabled() {
		t.Fatal("expected alias from reasoningAsContent")
	}
}
