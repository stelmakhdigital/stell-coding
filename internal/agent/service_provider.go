package agent

import (
	"github.com/stelmakhdigital/stell-ai/provider"
	"github.com/stelmakhdigital/stell-coding/internal/extensions"
	"github.com/stelmakhdigital/stell-coding/internal/themes"
)

// InitProviderOverrides инициализирует registry override провайдеров расширений из models конфига.
func (s *Service) InitProviderOverrides() {
	if s.Extensions == nil || s.Config == nil {
		return
	}
	s.Extensions.SetProviderBaseModels(s.Config.Models)
}

// ExtensionRegisterProvider применяет runtime override провайдера (хук registerProvider).
func (s *Service) ExtensionRegisterProvider(name string, cfg extensions.ProviderOverrideConfig, owner string) error {
	if s.Extensions == nil {
		return nil
	}
	if err := s.Extensions.RegisterProvider(name, cfg, owner); err != nil {
		return err
	}
	return s.rebuildModelsFromOverrides()
}

// ExtensionUnregisterProvider удаляет runtime override провайдера.
func (s *Service) ExtensionUnregisterProvider(name string) error {
	if s.Extensions == nil {
		return nil
	}
	s.Extensions.UnregisterProvider(name)
	return s.rebuildModelsFromOverrides()
}

func (s *Service) rebuildModelsFromOverrides() error {
	if s.Extensions == nil || s.Config == nil || s.Registry == nil {
		return nil
	}
	models := s.Extensions.EffectiveModels()
	if len(models) == 0 {
		return nil
	}
	s.Config.Models = models
	reg, err := provider.BuildRegistry(models, s.Config.Auth)
	if err != nil {
		return err
	}
	s.Registry = reg
	return nil
}

// ExtensionListThemes возвращает имена тем, доступных расширениям.
func (s *Service) ExtensionListThemes() []map[string]string {
	if s.Config == nil {
		return nil
	}
	list, err := themes.Resolve(themes.ResolveOpts{
		GlobalDir:  s.Config.GlobalDir,
		ProjectDir: s.Config.ProjectDir,
		Workspace:  s.Config.Workspace,
	})
	if err != nil {
		return nil
	}
	out := make([]map[string]string, 0, len(list))
	active := themes.ResolveThemeSetting(s.Config.Settings.Theme, themes.DetectDefaultName())
	if active == "" {
		active = s.Config.Settings.Theme
	}
	for _, t := range list {
		entry := map[string]string{"name": t.Name}
		if t.Path() != "" {
			entry["path"] = t.Path()
		}
		if t.Name == active {
			entry["active"] = "true"
		}
		out = append(out, entry)
	}
	if active != "" {
		found := false
		for _, e := range out {
			if e["name"] == active {
				found = true
				break
			}
		}
		if !found {
			out = append(out, map[string]string{"name": active, "active": "true"})
		}
	}
	return out
}
