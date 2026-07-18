package agent

import (
	"testing"

	"github.com/stelmakhdigital/stell-ai"
	"github.com/stelmakhdigital/stell-coding/internal/config"
)

func TestEstimateTokensAndBudget(t *testing.T) {
	msgs := []ai.Message{{Role: ai.RoleUser, Content: stringsRepeat("word ", 1000)}}
	tokens := EstimateTokens(msgs)
	if tokens < 200 {
		t.Fatalf("tokens=%d", tokens)
	}
	settings := config.DefaultSettings()
	mc := config.ModelConfig{ContextWindow: 8000, DefaultParams: config.InferenceParams{MaxTokens: 1000}}
	budget := contextBudget(mc, settings)
	if budget <= 0 {
		t.Fatalf("budget=%d", budget)
	}
	system := "system prompt"
	big := []ai.Message{{Role: ai.RoleUser, Content: stringsRepeat("word ", 8000)}}
	if !needsAutoCompact(system, big, nil, mc, settings, true) {
		t.Fatal("expected auto compact needed")
	}
	if needsAutoCompact(system, msgs, nil, mc, settings, true) {
		t.Fatal("small history should not trigger compaction")
	}
}

func TestContextBudgetLocalDefault(t *testing.T) {
	settings := config.DefaultSettings()
	local := config.ModelConfig{Local: true}
	remote := config.ModelConfig{}
	if lb, rb := contextBudget(local, settings), contextBudget(remote, settings); lb >= rb {
		t.Fatalf("local budget %d should be smaller than remote %d", lb, rb)
	}
}

func TestEstimateTokensCountsToolArgsAndImages(t *testing.T) {
	base := []ai.Message{{Role: ai.RoleAssistant, Content: "x"}}
	withArgs := []ai.Message{{Role: ai.RoleAssistant, Content: "x", ToolCalls: []ai.ToolCall{
		{Name: "bash", Args: map[string]any{"command": stringsRepeat("a", 400)}},
	}}}
	if EstimateTokens(withArgs) <= EstimateTokens(base)+16 {
		t.Fatal("tool args should add to the estimate")
	}
	withImage := []ai.Message{{Role: ai.RoleUser, Content: "x", Images: []ai.ImageContent{{Data: "zzz"}}}}
	if EstimateTokens(withImage) < perImageEstimateTokens {
		t.Fatal("images should add to the estimate")
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
