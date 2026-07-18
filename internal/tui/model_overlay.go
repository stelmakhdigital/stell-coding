package tui

import (
	"fmt"
	"strings"

	"stell/coding-agent/internal/config"
)

func (m *Model) availableModels() []config.ModelConfig {
	models := m.cfg.Models
	if len(m.cfg.Settings.EnabledModels) == 0 {
		return models
	}
	allowed := map[string]bool{}
	for _, name := range m.cfg.Settings.EnabledModels {
		allowed[name] = true
	}
	var out []config.ModelConfig
	for _, mod := range models {
		if allowed[mod.Name] {
			out = append(out, mod)
		}
	}
	if len(out) == 0 {
		return models
	}
	return out
}

func (m *Model) openModelOverlay() {
	m.openModelOverlayFiltered("")
}

func (m *Model) openModelOverlayFiltered(query string) {
	if m.cfg != nil {
		if models, err := config.ReloadModels(m.cfg.Workspace); err == nil {
			m.cfg.Models = models
		}
	}
	models := m.availableModels()
	m.modelNames = make([]string, 0, len(models))
	for _, mod := range models {
		label := mod.Name
		if mod.Local {
			label += " [local]"
			if !config.HasAuthConfigured(m.cfg.Auth, mod) && mod.APIKeyRef == "" {
				label += " [offline?]"
			}
		}
		m.modelNames = append(m.modelNames, label)
	}
	if query != "" {
		if hits := FuzzyFilter(query, m.modelNames, len(m.modelNames)); len(hits) == 0 {
			m.addError(fmt.Sprintf("no models matching %q", query))
			return
		}
	}
	list := NewSelectList(m.modelNames, func(picked string) {
		m.selectModel(picked)
		// closeOverlay выполняется в handleOverlayKey на текущей копии Update
		// (value-receiver Update делает closure момента открытия устаревшим).
	})
	if query != "" {
		list.SetFilter(query)
	}
	cursor := 0
	st := m.svc.GetState()
	for i, name := range list.Items {
		if strings.Split(name, " [")[0] == st.ModelName {
			cursor = i
			break
		}
	}
	list.Cursor = cursor
	list.Title = "models"
	list.Accent = colorSeq(m.colors.Accent)
	list.Muted = colorSeq(m.colors.Muted)
	m.pushOverlayFrame(overlayFrame{
		mode:        overlayModel,
		listKind:    "model",
		comp:        list,
		cursor:      cursor,
		modelNames:  append([]string(nil), m.modelNames...),
		overlayList: "model",
	})
	m.syncOverlayFromComp()
}

func renderModelOverlay(names []string, cursor int, current string) string {
	if len(names) == 0 {
		return "models: (none configured)\n(esc to close)"
	}
	var b strings.Builder
	b.WriteString("models (↑/↓ select, enter switch, esc close)\n")
	for i, name := range names {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		display := name
		currentBase := strings.Split(current, " [")[0]
		nameBase := strings.Split(name, " [")[0]
		line := prefix + display
		if nameBase == currentBase {
			line += " *"
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) selectModel(name string) {
	name = strings.TrimSuffix(strings.Split(name, " [")[0], "")
	for _, mod := range m.availableModels() {
		if mod.Name == name {
			m.svc.SetModelRecord(mod)
			m.addInfo("model → " + name)
			return
		}
	}
	m.addError(fmt.Sprintf("model %q not found", name))
}

func (m *Model) cycleModel() {
	name, err := m.svc.CycleModel()
	if err != nil {
		m.addError(err.Error())
		return
	}
	m.addInfo("model → " + name)
}

func (m *Model) cycleModelBackward() {
	models := m.availableModels()
	if len(models) == 0 {
		return
	}
	st := m.svc.GetState()
	idx := 0
	for i, mod := range models {
		if mod.Name == st.ModelName {
			idx = i
			break
		}
	}
	idx--
	if idx < 0 {
		idx = len(models) - 1
	}
	m.selectModel(models[idx].Name)
}
