package config

import (
	"strings"

	"github.com/stelmakhdigital/stell-ai"
)

const (
	// DefaultMaxTokens — лимит output-токенов модели по умолчанию.
	DefaultMaxTokens    = 16384
	MaxModelTokensCap = 32000
	minOutputTokens     = 1024
)

var defaultThinkingBudgets = map[string]int{
	"minimal": 1024,
	"low":     2048,
	"medium":  8192,
	"high":    16384,
}

// TokenBudget хранит скорректированные лимиты output и thinking токенов.
type TokenBudget struct {
	MaxTokens      int
	ThinkingBudget int
}

// ThinkingBudget возвращает бюджет токенов для уровня thinking из settings или default.
func ThinkingBudget(settings Settings, level string) int {
	if settings.ThinkingBudgets != nil {
		if v, ok := settings.ThinkingBudgets[level]; ok {
			return v
		}
	}
	level = ClampReasoningLevel(level)
	if level == "" {
		return 0
	}
	budgets := defaultThinkingBudgets
	if v, ok := budgets[level]; ok {
		return v
	}
	return 0
}

// ClampReasoningLevel отображает xhigh в high; возвращает "" для off-уровней.
func ClampReasoningLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "off", "none", "disabled":
		return ""
	case "xhigh":
		return "high"
	default:
		return strings.ToLower(strings.TrimSpace(level))
	}
}

// IsThinkingOff сообщает, нужно ли отключить reasoning/thinking.
func IsThinkingOff(level string) bool {
	return ClampReasoningLevel(level) == ""
}

// ModelMaxTokens возвращает настроенный max output токенов модели или default.
func ModelMaxTokens(mc ModelConfig) int {
	if mc.DefaultParams.MaxTokens > 0 {
		return mc.DefaultParams.MaxTokens
	}
	return DefaultMaxTokens
}

// AdjustMaxTokensForThinking корректирует max tokens под thinking budget.
func AdjustMaxTokensForThinking(baseMaxTokens, modelMaxTokens int, reasoningLevel string, customBudgets map[string]int) TokenBudget {
	budgets := map[string]int{
		"minimal": defaultThinkingBudgets["minimal"],
		"low":     defaultThinkingBudgets["low"],
		"medium":  defaultThinkingBudgets["medium"],
		"high":    defaultThinkingBudgets["high"],
	}
	for k, v := range customBudgets {
		budgets[k] = v
	}

	level := ClampReasoningLevel(reasoningLevel)
	if level == "" {
		level = "medium"
	}
	thinkingBudget := budgets[level]
	if thinkingBudget == 0 {
		thinkingBudget = defaultThinkingBudgets["medium"]
	}

	if modelMaxTokens <= 0 {
		modelMaxTokens = DefaultMaxTokens
	}
	if baseMaxTokens <= 0 {
		baseMaxTokens = modelMaxTokens
	}
	if baseMaxTokens > MaxModelTokensCap {
		baseMaxTokens = MaxModelTokensCap
	}

	maxTokens := baseMaxTokens + thinkingBudget
	if maxTokens > modelMaxTokens {
		maxTokens = modelMaxTokens
	}
	if maxTokens <= thinkingBudget {
		thinkingBudget = maxTokens - minOutputTokens
		if thinkingBudget < 0 {
			thinkingBudget = 0
		}
	}
	return TokenBudget{MaxTokens: maxTokens, ThinkingBudget: thinkingBudget}
}

// ChatTokenBudget вычисляет MaxTokens и ThinkingBudget для chat-запроса.
func ChatTokenBudget(mc ModelConfig, settings Settings, level string) (TokenBudget, string) {
	mapped, ok := MapThinkingLevel(mc, level)
	if !ok {
		level = "off"
	} else {
		level = mapped
	}
	modelMax := ModelMaxTokens(mc)
	baseMax := modelMax
	if baseMax > MaxModelTokensCap {
		baseMax = MaxModelTokensCap
	}
	if IsThinkingOff(level) {
		return TokenBudget{MaxTokens: baseMax, ThinkingBudget: 0}, level
	}
	return AdjustMaxTokensForThinking(baseMax, modelMax, level, settings.ThinkingBudgets), level
}

// MapThinkingLevel применяет per-model thinkingLevelMap, если настроен.
func MapThinkingLevel(mc ModelConfig, level string) (mapped string, ok bool) {
	if len(mc.ThinkingLevelMap) == 0 {
		return level, true
	}
	v, exists := mc.ThinkingLevelMap[level]
	if !exists {
		return level, true
	}
	if v == nil {
		return "", false
	}
	if *v == "" {
		return level, true
	}
	return *v, true
}


// HasAuthConfigured проверяет, вероятно ли у модели есть auth, без выполнения shell-команд.
func HasAuthConfigured(auth *Auth, mc ModelConfig) bool {
	return ai.HasAuthConfigured(auth, mc)
}

// SupportedThinkingLevels возвращает уровни thinking, поддерживаемые конфигом модели.
func SupportedThinkingLevels(mc ModelConfig) []string {
	if len(mc.ThinkingLevelMap) == 0 {
		return []string{"off", "low", "medium", "high"}
	}
	all := []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"}
	var out []string
	for _, lv := range all {
		if _, ok := MapThinkingLevel(mc, lv); ok {
			out = append(out, lv)
		}
	}
	if len(out) == 0 {
		return []string{"off", "low", "medium", "high"}
	}
	return out
}

