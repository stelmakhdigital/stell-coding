package config

import (
	"fmt"
	"strings"

	"github.com/stelmakhdigital/ai/provider"
)

// applyProviderConfig сливает настройки провайдера и modelOverrides на загруженные модели.
func applyProviderConfig(models []ModelConfig, providers map[string]ProviderConfig) ([]ModelConfig, error) {
	if len(providers) == 0 || len(models) == 0 {
		return models, nil
	}
	out := make([]ModelConfig, len(models))
	copy(out, models)
	for i, mc := range out {
		pid := providerKey(mc)
		pc, ok := providers[pid]
		if !ok {
			continue
		}
		out[i] = applyProviderLevel(mc, pc)
		modelKey := mc.Model
		if modelKey == "" {
			modelKey = mc.Name
		}
		if over, ok := pc.ModelOverrides[modelKey]; ok {
			patched, err := mergeModelOverride(out[i], pc, over)
			if err != nil {
				return nil, fmt.Errorf("model %q override: %w", mc.Name, err)
			}
			out[i] = patched
		}
	}
	return out, nil
}

func providerKey(mc ModelConfig) string {
	if mc.ProviderID != "" {
		return mc.ProviderID
	}
	return mc.Provider
}

func applyProviderLevel(mc ModelConfig, pc ProviderConfig) ModelConfig {
	if pc.BaseURL != "" {
		mc.APIBase = strings.TrimRight(pc.BaseURL, "/")
		mc.Local = provider.IsLocalEndpoint(mc.APIBase, mc.ProviderID)
	}
	if pc.API != "" {
		mc.APIType = pc.API
		mc.Provider = provider.APIToFactory(pc.API, providerKey(mc), mc.APIBase)
	}
	if pc.APIKey != "" && mc.APIKeyRef == "" {
		mc.APIKeyRef = pc.APIKey
	}
	mc.Compat = mergeCompat(pc.Compat, mc.Compat)
	if len(pc.Headers) > 0 {
		if mc.Headers == nil {
			mc.Headers = cloneHeaders(pc.Headers)
		}
	}
	if pc.AuthHeader != nil && *pc.AuthHeader {
		mc.AuthHeader = true
	}
	return mc
}

func mergeModelOverride(mc ModelConfig, pc ProviderConfig, over ProviderModelEntry) (ModelConfig, error) {
	if over.ID == "" {
		over.ID = mc.Model
	}
	patched, err := providerEntryToModel(providerKey(mc), pc, over)
	if err != nil {
		return mc, err
	}
	if over.Name == "" {
		patched.Name = mc.Name
	}
	return patched, nil
}
