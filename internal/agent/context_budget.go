package agent

import (
	"context"
	"encoding/json"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
)

const (
	perMessageOverheadTokens = 8
	// Грубая стоимость на изображение; фактическая зависит от модели и разрешения.
	perImageEstimateTokens = 1100
	// Консервативное окно для локальных моделей с неизвестным размером контекста:
	// завышение окна приводит к тихому переполнению реального, и
	// сервер обрезает начало истории.
	defaultLocalContextWindow = 8192
	defaultContextWindow      = 128000
)

func EstimateTokens(msgs []ai.Message) int {
	total := 0
	for _, m := range msgs {
		total += len(m.Content)/4 + perMessageOverheadTokens
		total += len(m.Images) * perImageEstimateTokens
		for _, tc := range m.ToolCalls {
			total += len(tc.Name)/4 + 16
			if len(tc.Args) > 0 {
				if b, err := json.Marshal(tc.Args); err == nil {
					total += len(b) / 4
				}
			}
		}
	}
	return total
}

func EstimateToolDefTokens(defs []ai.ToolDef) int {
	total := 0
	for _, d := range defs {
		total += (len(d.Name)+len(d.Description))/4 + 16
		if len(d.Parameters) > 0 {
			if b, err := json.Marshal(d.Parameters); err == nil {
				total += len(b) / 4
			}
		}
	}
	return total
}

func contextBudget(cfg config.ModelConfig, settings config.Settings) int {
	window := cfg.ContextWindow
	if window <= 0 {
		if cfg.Local {
			window = defaultLocalContextWindow
		} else {
			window = defaultContextWindow
		}
	}
	reserve := settings.Compaction.ReserveTokens
	if reserve <= 0 {
		reserve = 16384
	}
	// Значения выше настроены для больших окон; масштабируем вниз для
	// малых, чтобы бюджет не схлопнулся до минимума.
	if reserve > window/4 {
		reserve = window / 4
	}
	maxOut := config.ModelMaxTokens(cfg)
	if maxOut <= 0 {
		maxOut = config.DefaultMaxTokens
	}
	if maxOut > window/4 {
		maxOut = window / 4
	}
	budget := window - reserve - maxOut
	if budget < 1024 {
		return 1024
	}
	return budget
}

func needsAutoCompact(system string, history []ai.Message, toolDefs []ai.ToolDef, cfg config.ModelConfig, settings config.Settings, enabled bool) bool {
	if !enabled || !settings.CompactionEnabled() {
		return false
	}
	msgs := buildModelMessages(system, history)
	return EstimateTokens(msgs)+EstimateToolDefTokens(toolDefs) > contextBudget(cfg, settings)
}

func (a *Agent) maybeAutoCompact(ctx context.Context) error {
	system := a.buildSystem(ctx, "")
	svc := &Service{
		Config: a.Config, Registry: a.Registry, Tools: a.Tools,
		Sessions: a.Sessions, SessPath: a.SessPath, Model: a.Model,
		Catalog: a.Catalog, Hooks: a.Hooks,
	}
	var toolDefs []ai.ToolDef
	if a.Tools != nil {
		toolDefs = a.Tools.Defs()
	}
	enabled := a.Config.Settings.CompactionEnabled()
	if a.AutoCompactionEnabled != nil {
		enabled = a.AutoCompactionEnabled()
	}
	if !needsAutoCompact(system, a.Sessions.BuildMessages(), toolDefs, a.Model, a.Config.Settings, enabled) {
		return nil
	}
	if a.CompactionEmitter != nil {
		a.CompactionEmitter(true, "threshold", nil)
	}
	info, err := svc.Compact(ctx)
	if a.CompactionEmitter != nil {
		if err != nil {
			a.CompactionEmitter(false, "", map[string]any{"error": err.Error()})
		} else {
			a.CompactionEmitter(false, "", info)
		}
	}
	return err
}
