package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stelmakhdigital/ai"
	"github.com/stelmakhdigital/ai/provider"
)

// ProviderConfig — блок provider в models.json (чтение/запись).
type ProviderConfig struct {
	BaseURL        string            `json:"baseUrl,omitempty"`
	API            string            `json:"api,omitempty"`
	APIKey         string            `json:"apiKey,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	AuthHeader     *bool             `json:"authHeader,omitempty"`
	Compat         CompatSettings    `json:"compat,omitempty"`
	Models         []ProviderModelEntry    `json:"models,omitempty"`
	ModelOverrides map[string]ProviderModelEntry `json:"modelOverrides,omitempty"`
}

// ProviderModelEntry — модель внутри блока provider.
type ProviderModelEntry struct {
	ID               string            `json:"id"`
	Name             string            `json:"name,omitempty"`
	API              string            `json:"api,omitempty"`
	Reasoning        *bool             `json:"reasoning,omitempty"`
	ThinkingLevelMap map[string]*string `json:"thinkingLevelMap,omitempty"`
	Input            []string          `json:"input,omitempty"`
	ContextWindow    int               `json:"contextWindow,omitempty"`
	MaxTokens        int               `json:"maxTokens,omitempty"`
	Cost             *ModelCost           `json:"cost,omitempty"`
	Compat           CompatSettings    `json:"compat,omitempty"`
}

// ModelCost — цена за миллион токенов в models.json.
type ModelCost struct {
	Input      float64       `json:"input"`
	Output     float64       `json:"output"`
	CacheRead  float64       `json:"cacheRead,omitempty"`
	CacheWrite float64       `json:"cacheWrite,omitempty"`
	Tiers      []ModelCostTier  `json:"tiers,omitempty"`
}

// ModelCostTier — альтернативная цена, когда input-токены превышают порог.
type ModelCostTier struct {
	InputTokensAbove int     `json:"inputTokensAbove"`
	Input            float64 `json:"input"`
	Output           float64 `json:"output"`
	CacheRead        float64 `json:"cacheRead,omitempty"`
	CacheWrite       float64 `json:"cacheWrite,omitempty"`
}

type CompatSettings = ai.CompatSettings

type rawModelsFile struct {
	Models    []ModelConfig              `json:"models,omitempty"`
	Providers map[string]ProviderConfig `json:"providers,omitempty"`
}

// ParseModelsJSON загружает legacy models[] и/или providers{} в ModelsFile.
func ParseModelsJSON(data []byte) (ModelsFile, error) {
	var raw rawModelsFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return ModelsFile{}, err
	}
	out := ModelsFile{
		Providers: raw.Providers,
		Format:    detectModelsFormat(raw),
	}
	if len(raw.Models) > 0 {
		out.Models = append(out.Models, raw.Models...)
	}
	if len(raw.Providers) > 0 {
		providerModels, err := expandProviders(raw.Providers)
		if err != nil {
			return ModelsFile{}, err
		}
		out.Models = mergeModelList(out.Models, providerModels)
		out.Models, err = applyProviderConfig(out.Models, raw.Providers)
		if err != nil {
			return ModelsFile{}, err
		}
	}
	return out, nil
}

func detectModelsFormat(raw rawModelsFile) string {
	if len(raw.Providers) > 0 {
		return "providers"
	}
	return "legacy"
}

func mergeModelList(base, extra []ModelConfig) []ModelConfig {
	if len(extra) == 0 {
		return base
	}
	byName := map[string]ModelConfig{}
	order := []string{}
	for _, m := range base {
		byName[m.Name] = m
		order = append(order, m.Name)
	}
	for _, m := range extra {
		if _, ok := byName[m.Name]; !ok {
			order = append(order, m.Name)
		}
		byName[m.Name] = m
	}
	out := make([]ModelConfig, 0, len(order))
	for _, name := range order {
		out = append(out, byName[name])
	}
	return out
}

func expandProviders(providers map[string]ProviderConfig) ([]ModelConfig, error) {
	var out []ModelConfig
	keys := sortedKeys(providers)
	for _, providerID := range keys {
		pc := providers[providerID]
		for _, entry := range pc.Models {
			mc, err := providerEntryToModel(providerID, pc, entry)
			if err != nil {
				return nil, err
			}
			out = append(out, mc)
		}
	}
	return out, nil
}

func providerEntryToModel(providerID string, pc ProviderConfig, entry ProviderModelEntry) (ModelConfig, error) {
	if entry.ID == "" {
		return ModelConfig{}, fmt.Errorf("provider %q: model id is required", providerID)
	}
	api := entry.API
	if api == "" {
		api = pc.API
	}
	base := strings.TrimRight(pc.BaseURL, "/")
	providerType := provider.APIToFactory(api, providerID, base)
	name := entry.Name
	if name == "" {
		name = entry.ID
	}
	compat := mergeCompat(pc.Compat, entry.Compat)
	reasoning := false
	if entry.Reasoning != nil {
		reasoning = *entry.Reasoning
	}
	mc := ModelConfig{
		Name:               name,
		Provider:           providerType,
		ProviderID:         providerID,
		Model:              entry.ID,
		APIBase:            base,
		APIType:            api,
		APIKeyRef:          pc.APIKey,
		ContextWindow:      entry.ContextWindow,
		Compat:             compat,
		Reasoning:          reasoning,
		Local:              provider.IsLocalEndpoint(base, providerID),
		ThinkingLevelMap:   entry.ThinkingLevelMap,
		Headers:            cloneHeaders(pc.Headers),
		AuthHeader:         pc.AuthHeader != nil && *pc.AuthHeader,
		DefaultParams:      InferenceParams{MaxTokens: entry.MaxTokens},
	}
	if entry.Cost != nil {
		mc.InputCostPerM = entry.Cost.Input
		mc.OutputCostPerM = entry.Cost.Output
		for _, t := range entry.Cost.Tiers {
			mc.CostTiers = append(mc.CostTiers, CostTier{
				InputTokensAbove: t.InputTokensAbove,
				InputPerM:        t.Input,
				OutputPerM:       t.Output,
				CacheReadPerM:    t.CacheRead,
				CacheWritePerM:   t.CacheWrite,
			})
		}
	}
	if len(entry.Input) > 0 {
		mc.Input = append([]string(nil), entry.Input...)
	}
	return mc, nil
}

func mergeCompat(base, over CompatSettings) CompatSettings {
	out := base
	if over.SupportsDeveloperRole != nil {
		out.SupportsDeveloperRole = over.SupportsDeveloperRole
	}
	if over.SupportsReasoningEffort != nil {
		out.SupportsReasoningEffort = over.SupportsReasoningEffort
	}
	if over.ReasoningAsContent != nil {
		out.ReasoningAsContent = over.ReasoningAsContent
	}
	if over.ThinkingFormat != nil {
		out.ThinkingFormat = over.ThinkingFormat
	}
	if over.RequiresThinkingAsText != nil {
		out.RequiresThinkingAsText = over.RequiresThinkingAsText
	}
	if over.RequiresReasoningContentOnAssistantMessages != nil {
		out.RequiresReasoningContentOnAssistantMessages = over.RequiresReasoningContentOnAssistantMessages
	}
	if over.SupportsUsageInStreaming != nil {
		out.SupportsUsageInStreaming = over.SupportsUsageInStreaming
	}
	if over.MaxTokensField != nil {
		out.MaxTokensField = over.MaxTokensField
	}
	if over.AllowEmptySignature != nil {
		out.AllowEmptySignature = over.AllowEmptySignature
	}
	if over.ChatTemplateKwargs != nil {
		out.ChatTemplateKwargs = CloneCompatMap(over.ChatTemplateKwargs)
	}
	if over.SupportsStore != nil {
		out.SupportsStore = over.SupportsStore
	}
	if over.RequiresToolResultName != nil {
		out.RequiresToolResultName = over.RequiresToolResultName
	}
	if over.RequiresAssistantAfterToolResult != nil {
		out.RequiresAssistantAfterToolResult = over.RequiresAssistantAfterToolResult
	}
	if over.SupportsStrictMode != nil {
		out.SupportsStrictMode = over.SupportsStrictMode
	}
	if over.CacheControlFormat != nil {
		out.CacheControlFormat = over.CacheControlFormat
	}
	if over.SendSessionAffinityHeaders != nil {
		out.SendSessionAffinityHeaders = over.SendSessionAffinityHeaders
	}
	if over.SessionAffinityFormat != nil {
		out.SessionAffinityFormat = over.SessionAffinityFormat
	}
	if over.SupportsLongCacheRetention != nil {
		out.SupportsLongCacheRetention = over.SupportsLongCacheRetention
	}
	if over.OpenRouterRouting != nil {
		out.OpenRouterRouting = CloneCompatMap(over.OpenRouterRouting)
	}
	if over.VercelGatewayRouting != nil {
		out.VercelGatewayRouting = CloneCompatMap(over.VercelGatewayRouting)
	}
	if over.SupportsEagerToolInputStreaming != nil {
		out.SupportsEagerToolInputStreaming = over.SupportsEagerToolInputStreaming
	}
	if over.ForceAdaptiveThinking != nil {
		out.ForceAdaptiveThinking = over.ForceAdaptiveThinking
	}
	if over.SupportsCacheControlOnTools != nil {
		out.SupportsCacheControlOnTools = over.SupportsCacheControlOnTools
	}
	return out
}

func cloneHeaders(h map[string]string) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// MarshalModelsFile пишет формат providers, если они заданы, иначе legacy models[].
func MarshalModelsFile(m ModelsFile) ([]byte, error) {
	if len(m.Providers) > 0 || m.Format == "providers" {
		if m.Providers == nil {
			m.Providers = modelsToProviders(m.Models)
		}
		payload := struct {
			Providers map[string]ProviderConfig `json:"providers"`
		}{Providers: m.Providers}
		return json.MarshalIndent(payload, "", "  ")
	}
	payload := struct {
		Models []ModelConfig `json:"models"`
	}{Models: m.Models}
	return json.MarshalIndent(payload, "", "  ")
}

// modelsToProviders группирует нормализованные модели обратно в providers для сохранения.
func modelsToProviders(models []ModelConfig) map[string]ProviderConfig {
	providers := map[string]ProviderConfig{}
	for _, mc := range models {
		pid := mc.ProviderID
		if pid == "" {
			pid = mc.Provider
		}
		pc := providers[pid]
		if pc.BaseURL == "" {
			pc.BaseURL = mc.APIBase
		}
		if pc.API == "" {
			pc.API = mc.APIType
			if pc.API == "" {
				pc.API = provider.FactoryToAPI(mc.Provider)
			}
		}
		if pc.APIKey == "" {
			pc.APIKey = mc.APIKeyRef
		}
		if pc.Headers == nil && len(mc.Headers) > 0 {
			pc.Headers = cloneHeaders(mc.Headers)
		}
		if mc.AuthHeader && pc.AuthHeader == nil {
			v := true
			pc.AuthHeader = &v
		}
		pc.Compat = mergeProviderCompat(pc.Compat, mc.Compat)
		entry := ProviderModelEntry{
			ID:               mc.Model,
			Name:             displayName(mc),
			ContextWindow:    mc.ContextWindow,
			MaxTokens:        mc.DefaultParams.MaxTokens,
			ThinkingLevelMap: mc.ThinkingLevelMap,
			Compat:           mc.Compat,
		}
		if mc.Reasoning {
			v := true
			entry.Reasoning = &v
		}
		if len(mc.Input) > 0 {
			entry.Input = append([]string(nil), mc.Input...)
		}
		if mc.InputCostPerM > 0 || mc.OutputCostPerM > 0 {
			entry.Cost = &ModelCost{Input: mc.InputCostPerM, Output: mc.OutputCostPerM}
		}
		pc.Models = append(pc.Models, entry)
		providers[pid] = pc
	}
	return providers
}

func mergeProviderCompat(base, mc CompatSettings) CompatSettings {
	out := base
	if mc.SupportsDeveloperRole != nil && base.SupportsDeveloperRole == nil {
		out.SupportsDeveloperRole = mc.SupportsDeveloperRole
	}
	if mc.SupportsReasoningEffort != nil && base.SupportsReasoningEffort == nil {
		out.SupportsReasoningEffort = mc.SupportsReasoningEffort
	}
	if mc.ReasoningAsContent != nil && base.ReasoningAsContent == nil {
		out.ReasoningAsContent = mc.ReasoningAsContent
	}
	if mc.ThinkingFormat != nil && base.ThinkingFormat == nil {
		out.ThinkingFormat = mc.ThinkingFormat
	}
	if mc.RequiresThinkingAsText != nil && base.RequiresThinkingAsText == nil {
		out.RequiresThinkingAsText = mc.RequiresThinkingAsText
	}
	if mc.RequiresReasoningContentOnAssistantMessages != nil && base.RequiresReasoningContentOnAssistantMessages == nil {
		out.RequiresReasoningContentOnAssistantMessages = mc.RequiresReasoningContentOnAssistantMessages
	}
	if mc.SupportsUsageInStreaming != nil && base.SupportsUsageInStreaming == nil {
		out.SupportsUsageInStreaming = mc.SupportsUsageInStreaming
	}
	if mc.MaxTokensField != nil && base.MaxTokensField == nil {
		out.MaxTokensField = mc.MaxTokensField
	}
	if mc.AllowEmptySignature != nil && base.AllowEmptySignature == nil {
		out.AllowEmptySignature = mc.AllowEmptySignature
	}
	if mc.ChatTemplateKwargs != nil && base.ChatTemplateKwargs == nil {
		out.ChatTemplateKwargs = CloneCompatMap(mc.ChatTemplateKwargs)
	}
	if mc.SupportsStore != nil && base.SupportsStore == nil {
		out.SupportsStore = mc.SupportsStore
	}
	if mc.RequiresToolResultName != nil && base.RequiresToolResultName == nil {
		out.RequiresToolResultName = mc.RequiresToolResultName
	}
	if mc.RequiresAssistantAfterToolResult != nil && base.RequiresAssistantAfterToolResult == nil {
		out.RequiresAssistantAfterToolResult = mc.RequiresAssistantAfterToolResult
	}
	if mc.SupportsStrictMode != nil && base.SupportsStrictMode == nil {
		out.SupportsStrictMode = mc.SupportsStrictMode
	}
	if mc.CacheControlFormat != nil && base.CacheControlFormat == nil {
		out.CacheControlFormat = mc.CacheControlFormat
	}
	if mc.SendSessionAffinityHeaders != nil && base.SendSessionAffinityHeaders == nil {
		out.SendSessionAffinityHeaders = mc.SendSessionAffinityHeaders
	}
	if mc.SessionAffinityFormat != nil && base.SessionAffinityFormat == nil {
		out.SessionAffinityFormat = mc.SessionAffinityFormat
	}
	if mc.SupportsLongCacheRetention != nil && base.SupportsLongCacheRetention == nil {
		out.SupportsLongCacheRetention = mc.SupportsLongCacheRetention
	}
	if mc.OpenRouterRouting != nil && base.OpenRouterRouting == nil {
		out.OpenRouterRouting = CloneCompatMap(mc.OpenRouterRouting)
	}
	if mc.VercelGatewayRouting != nil && base.VercelGatewayRouting == nil {
		out.VercelGatewayRouting = CloneCompatMap(mc.VercelGatewayRouting)
	}
	if mc.SupportsEagerToolInputStreaming != nil && base.SupportsEagerToolInputStreaming == nil {
		out.SupportsEagerToolInputStreaming = mc.SupportsEagerToolInputStreaming
	}
	if mc.ForceAdaptiveThinking != nil && base.ForceAdaptiveThinking == nil {
		out.ForceAdaptiveThinking = mc.ForceAdaptiveThinking
	}
	if mc.SupportsCacheControlOnTools != nil && base.SupportsCacheControlOnTools == nil {
		out.SupportsCacheControlOnTools = mc.SupportsCacheControlOnTools
	}
	return out
}

func displayName(mc ModelConfig) string {
	if mc.Name != "" && mc.Name != mc.Model {
		return mc.Name
	}
	return ""
}

// AddProviderModel добавляет модель в блок providers (создаёт provider при необходимости).
func (m *ModelsFile) AddProviderModel(providerID string, entry ProviderModelEntry, defaults ProviderConfig) error {
	if entry.ID == "" {
		return fmt.Errorf("model id is required")
	}
	if m.Providers == nil {
		m.Providers = map[string]ProviderConfig{}
	}
	pc := m.Providers[providerID]
	if pc.BaseURL == "" {
		pc.BaseURL = defaults.BaseURL
	}
	if pc.API == "" {
		pc.API = defaults.API
	}
	if pc.APIKey == "" {
		pc.APIKey = defaults.APIKey
	}
	if pc.Compat.SupportsDeveloperRole == nil && defaults.Compat.SupportsDeveloperRole != nil {
		pc.Compat = defaults.Compat
	}
	for _, existing := range pc.Models {
		if existing.ID == entry.ID {
			return fmt.Errorf("model %q already exists in provider %q", entry.ID, providerID)
		}
	}
	pc.Models = append(pc.Models, entry)
	m.Providers[providerID] = pc
	m.Format = "providers"

	mc, err := providerEntryToModel(providerID, pc, entry)
	if err != nil {
		return err
	}
	m.Models = mergeModelList(m.Models, []ModelConfig{mc})
	return nil
}

// LoadModelsFile читает models.json по path.
func LoadModelsFile(path string) (ModelsFile, error) {
	b, err := readFileBytes(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ModelsFile{Format: "providers", Providers: map[string]ProviderConfig{}}, nil
		}
		return ModelsFile{}, err
	}
	return ParseModelsJSON(b)
}

// SaveModelsFile пишет models.json в формате providers, когда уместно.
func SaveModelsFile(path string, m ModelsFile) error {
	data, err := MarshalModelsFile(m)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileBytes(path, data, 0o644)
}

// GlobalModelsPath возвращает ~/.stell/agent/models.json.
func GlobalModelsPath() (string, error) {
	dir, err := GlobalDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "models.json"), nil
}
