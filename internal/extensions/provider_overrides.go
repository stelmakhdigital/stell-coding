package extensions

import (
	"fmt"
	"sync"

	"github.com/stelmakhdigital/stell-coding/internal/config"
)

// ProviderModelDef — одна модель в динамической регистрации provider.
type ProviderModelDef struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	API           string `json:"api,omitempty"`
	ContextWindow int    `json:"contextWindow,omitempty"`
}

// ProviderOverrideConfig — payload запроса host/register_provider.
type ProviderOverrideConfig struct {
	BaseURL string             `json:"baseUrl,omitempty"`
	APIKey  string             `json:"apiKey,omitempty"`
	Headers map[string]string  `json:"headers,omitempty"`
	API     string             `json:"api,omitempty"`
	Models  []ProviderModelDef `json:"models,omitempty"`
}

type registeredProvider struct {
	Name   string
	Config ProviderOverrideConfig
	Owner  string
}

// ProviderOverrides хранит runtime-оверрайды factory/base URL провайдеров.
type ProviderOverrides struct {
	mu       sync.Mutex
	byName   map[string]registeredProvider
	baseSnap []config.ModelConfig
}

func NewProviderOverrides(base []config.ModelConfig) *ProviderOverrides {
	cp := make([]config.ModelConfig, len(base))
	copy(cp, base)
	return &ProviderOverrides{byName: map[string]registeredProvider{}, baseSnap: cp}
}

func (p *ProviderOverrides) Register(name string, cfg ProviderOverrideConfig, owner string) error {
	if name == "" {
		return fmt.Errorf("provider name required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.byName[name] = registeredProvider{Name: name, Config: cfg, Owner: owner}
	p.rebuildLocked()
	return nil
}

func (p *ProviderOverrides) Unregister(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.byName, name)
	p.rebuildLocked()
}

func (p *ProviderOverrides) UnregisterOwner(owner string) {
	if owner == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for name, rp := range p.byName {
		if rp.Owner == owner {
			delete(p.byName, name)
		}
	}
	p.rebuildLocked()
}

// Models возвращает итоговый список моделей с применёнными оверрайдами.
func (p *ProviderOverrides) Models() []config.ModelConfig {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]config.ModelConfig, len(p.baseSnap))
	copy(out, p.baseSnap)
	return applyOverrides(out, p.byName)
}

func (p *ProviderOverrides) rebuildLocked() {}

func applyOverrides(base []config.ModelConfig, overrides map[string]registeredProvider) []config.ModelConfig {
	out := make([]config.ModelConfig, len(base))
	copy(out, base)
	for i, mc := range out {
		rp, ok := overrides[mc.Provider]
		if !ok {
			continue
		}
		if rp.Config.BaseURL != "" {
			out[i].APIBase = rp.Config.BaseURL
		}
		if len(rp.Config.Headers) > 0 {
			out[i].Headers = mergeHeaders(out[i].Headers, rp.Config.Headers)
		}
	}
	out = append(out, addedModelsFromOverrides(overrides)...)
	return out
}

func addedModelsFromOverrides(overrides map[string]registeredProvider) []config.ModelConfig {
	var out []config.ModelConfig
	for name, rp := range overrides {
		if len(rp.Config.Models) == 0 {
			continue
		}
		for _, md := range rp.Config.Models {
			mc := config.ModelConfig{
				Name:          firstNonEmpty(md.Name, md.ID),
				Provider:      name,
				Model:         md.ID,
				APIBase:       rp.Config.BaseURL,
				APIType:       firstNonEmpty(md.API, rp.Config.API),
				ContextWindow: md.ContextWindow,
				Headers:       cloneHeaders(rp.Config.Headers),
			}
			if mc.ContextWindow == 0 {
				mc.ContextWindow = 128000
			}
			out = append(out, mc)
		}
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

func mergeHeaders(base, over map[string]string) map[string]string {
	out := cloneHeaders(base)
	if out == nil {
		out = map[string]string{}
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
