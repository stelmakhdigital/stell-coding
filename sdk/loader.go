package sdk

import (
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-coding/internal/discovery"
)

// ResourceDiagnostic — предупреждение/ошибка discovery (диагностика resource loader).
type ResourceDiagnostic struct {
	Type    string // warning | error | collision
	Message string
	Path    string
}

// ResourceLoader обнаруживает skills/prompts (и ресурсы пакетов) для workspace.
type ResourceLoader interface {
	Reload() error
	Catalog() *discovery.Catalog
	Diagnostics() []ResourceDiagnostic
}

// DefaultResourceLoader сканирует global/project package и каталоги agents skills.
type DefaultResourceLoader struct {
	cfg         *config.Config
	catalog     *discovery.Catalog
	diagnostics []ResourceDiagnostic
}

// NewDefaultResourceLoader создаёт loader, привязанный к cfg.
func NewDefaultResourceLoader(cfg *config.Config) *DefaultResourceLoader {
	return &DefaultResourceLoader{cfg: cfg}
}

// Reload пересканирует skills/prompts.
func (l *DefaultResourceLoader) Reload() error {
	if l.cfg == nil {
		return nil
	}
	cat, err := discovery.Load(l.cfg)
	if err != nil {
		l.diagnostics = []ResourceDiagnostic{{Type: "error", Message: err.Error()}}
		return err
	}
	l.catalog = cat
	l.diagnostics = nil
	seen := map[string]bool{}
	if cat != nil && cat.Skills != nil {
		for _, e := range cat.Skills.List() {
			if seen[e.Name] {
				l.diagnostics = append(l.diagnostics, ResourceDiagnostic{
					Type: "collision", Message: "duplicate skill " + e.Name, Path: e.Name,
				})
			}
			seen[e.Name] = true
		}
	}
	return nil
}

// Catalog возвращает последний загруженный каталог (может быть nil до Reload).
func (l *DefaultResourceLoader) Catalog() *discovery.Catalog {
	return l.catalog
}

// Diagnostics возвращает мягкие предупреждения с последнего Reload.
func (l *DefaultResourceLoader) Diagnostics() []ResourceDiagnostic {
	return append([]ResourceDiagnostic{}, l.diagnostics...)
}
