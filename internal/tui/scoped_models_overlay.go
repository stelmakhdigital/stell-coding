package tui

import (
	"fmt"
	"sort"
	"strings"

	"stell/coding-agent/internal/config"
)

func (m *Model) openScopedModelsOverlay() {
	m.scopedEnabled = map[string]bool{}
	for _, name := range m.cfg.Settings.EnabledModels {
		m.scopedEnabled[name] = true
	}
	if len(m.scopedEnabled) == 0 {
		for _, mc := range m.cfg.Models {
			m.scopedEnabled[mc.Name] = true
		}
	}
	m.modelNames = make([]string, 0, len(m.cfg.Models))
	for _, mc := range m.cfg.Models {
		if m.scopedProvider != "" && mc.Provider != m.scopedProvider {
			continue
		}
		m.modelNames = append(m.modelNames, mc.Name)
	}
	m.pushOverlayFrame(overlayFrame{
		mode:        overlayModel,
		listKind:    "scoped",
		text:        m.renderScopedModelsOverlay(),
		cursor:      0,
		modelNames:  append([]string(nil), m.modelNames...),
		overlayList: "scoped",
	})
}

func (m *Model) providerForModel(name string) string {
	for _, mc := range m.cfg.Models {
		if mc.Name == name {
			return mc.Provider
		}
	}
	return ""
}

func (m *Model) renderScopedModelsOverlay() string {
	var b strings.Builder
	b.WriteString("scoped models (space toggle, ctrl+p provider, alt+↑↓ reorder, ctrl+s save)\n")
	if m.scopedProvider != "" {
		fmt.Fprintf(&b, "  provider filter: %s\n", m.scopedProvider)
	}
	for i, name := range m.modelNames {
		prefix := "  "
		if i == m.overlayCursor {
			prefix = "> "
		}
		mark := "[ ]"
		if m.scopedEnabled[name] {
			mark = "[x]"
		}
		fmt.Fprintf(&b, "%s%s %s\n", prefix, mark, name)
	}
	return b.String()
}

func (m *Model) handleScopedModelsKey(key string) bool {
	if m.overlayList != "scoped" {
		return false
	}
	switch key {
	case " ":
		name := m.modelNames[m.overlayCursor]
		m.scopedEnabled[name] = !m.scopedEnabled[name]
		m.overlay = m.renderScopedModelsOverlay()
		return true
	case "ctrl+p":
		if len(m.modelNames) == 0 {
			return true
		}
		name := m.modelNames[m.overlayCursor]
		prov := m.providerForModel(name)
		if m.scopedProvider == prov {
			m.scopedProvider = ""
		} else {
			m.scopedProvider = prov
		}
		m.openScopedModelsOverlay()
		return true
	case "alt+up":
		if m.overlayCursor > 0 {
			i := m.overlayCursor
			m.modelNames[i], m.modelNames[i-1] = m.modelNames[i-1], m.modelNames[i]
			m.overlayCursor--
			m.overlay = m.renderScopedModelsOverlay()
		}
		return true
	case "alt+down":
		if m.overlayCursor < len(m.modelNames)-1 {
			i := m.overlayCursor
			m.modelNames[i], m.modelNames[i+1] = m.modelNames[i+1], m.modelNames[i]
			m.overlayCursor++
			m.overlay = m.renderScopedModelsOverlay()
		}
		return true
	case "ctrl+s":
		return m.saveScopedModels()
	case "ctrl+a":
		for _, name := range m.modelNames {
			m.scopedEnabled[name] = true
		}
		m.overlay = m.renderScopedModelsOverlay()
		return true
	case "ctrl+x":
		for _, name := range m.modelNames {
			m.scopedEnabled[name] = false
		}
		m.overlay = m.renderScopedModelsOverlay()
		return true
	case "esc":
		m.closeOverlay()
		return true
	case "enter":
		name := m.modelNames[m.overlayCursor]
		m.scopedEnabled[name] = !m.scopedEnabled[name]
		m.overlay = m.renderScopedModelsOverlay()
		return true
	}
	return false
}

func (m *Model) saveScopedModels() bool {
	var enabled []string
	for _, name := range m.modelNames {
		if m.scopedEnabled[name] {
			enabled = append(enabled, name)
		}
	}
	m.cfg.Settings.EnabledModels = enabled
	if err := config.SaveGlobalSettings(m.cfg.GlobalDir, m.cfg.Settings); err != nil {
		m.addError(err.Error())
		return true
	}
	m.addInfo(fmt.Sprintf("saved %d enabled models", len(enabled)))
	m.closeOverlay()
	m.syncViewport()
	return true
}

func (m *Model) sortSessionItems(items []sessionItem) []sessionItem {
	out := append([]sessionItem(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		if m.sessionSortDesc {
			return out[i].modTime.After(out[j].modTime)
		}
		return out[i].modTime.Before(out[j].modTime)
	})
	if m.sessionNamedOnly {
		var filtered []sessionItem
		for _, it := range out {
			if strings.Contains(it.label, "_") || strings.Contains(it.path, "named") {
				filtered = append(filtered, it)
			}
		}
		if len(filtered) > 0 {
			out = filtered
		}
	}
	return out
}

func (m *Model) handleSessionOverlayKey(key string) bool {
	if action, ok := m.overlayKeyAction(key); ok {
		return m.dispatchOverlayKeyAction(action)
	}
	return false
}
