package config

import (
	"github.com/stelmakhdigital/ai"
	"fmt"
	"os"
	"path/filepath"
)

type ModelsFile struct {
	Models    []ModelConfig              `json:"models"`
	Providers map[string]ProviderConfig `json:"-"`
	Format    string                      `json:"-"` // "legacy" | "providers"
}

type ModelConfig = ai.ModelConfig
type CostTier = ai.CostTier
type InferenceParams = ai.InferenceParams


func (m ModelsFile) Validate() error {
	seen := map[string]bool{}
	for _, mc := range m.Models {
		if mc.Name == "" {
			return fmt.Errorf("model name is required")
		}
		if mc.Provider == "" {
			return fmt.Errorf("model %q: provider is required", mc.Name)
		}
		if seen[mc.Name] {
			return fmt.Errorf("duplicate model name %q", mc.Name)
		}
		if mc.Model == "" {
			return fmt.Errorf("model %q: model id is required", mc.Name)
		}
		if mc.ContextWindow < 0 {
			return fmt.Errorf("model %q: contextWindow must be >= 0", mc.Name)
		}
		seen[mc.Name] = true
	}
	return nil
}

func mergeModels(base, over ModelsFile) ModelsFile {
	byName := map[string]ModelConfig{}
	order := []string{}
	for _, m := range base.Models {
		byName[m.Name] = m
		order = append(order, m.Name)
	}
	for _, m := range over.Models {
		if _, ok := byName[m.Name]; !ok {
			order = append(order, m.Name)
		}
		byName[m.Name] = m
	}
	out := ModelsFile{
		Models: make([]ModelConfig, 0, len(order)),
		Format: over.Format,
	}
	if len(over.Providers) > 0 {
		out.Providers = over.Providers
		out.Format = "providers"
	} else if base.Format == "providers" && len(base.Providers) > 0 {
		out.Providers = base.Providers
		out.Format = "providers"
	}
	for _, name := range order {
		out.Models = append(out.Models, byName[name])
	}
	return out
}

func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func writeFileBytes(path string, data []byte, mode os.FileMode) error {
	return os.WriteFile(path, data, mode)
}

func readModels(path string) (ModelsFile, bool, error) {
	b, err := readFileBytes(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ModelsFile{}, false, nil
		}
		return ModelsFile{}, false, err
	}
	m, err := ParseModelsJSON(b)
	if err != nil {
		return ModelsFile{}, false, fmt.Errorf("%s: %w", path, err)
	}
	return m, true, nil
}

// ReloadModels перечитывает global и project models.json для workspace.
func ReloadModels(workspace string) ([]ModelConfig, error) {
	globalDir, err := GlobalDir()
	if err != nil {
		return nil, err
	}
	var models ModelsFile
	if m, ok, err := readModels(filepath.Join(globalDir, "models.json")); err != nil {
		return nil, err
	} else if ok {
		models = m
	}
	projectStell := filepath.Join(workspace, ".stell")
	if info, err := os.Stat(projectStell); err == nil && info.IsDir() {
		if pm, ok, err := readModels(filepath.Join(projectStell, "models.json")); err != nil {
			return nil, err
		} else if ok {
			models = mergeModels(models, pm)
		}
	}
	if err := models.Validate(); err != nil {
		return nil, fmt.Errorf("models: %w", err)
	}
	return models.Models, nil
}
