package tui

const (
	actionTreeFilterDefault  = "treeFilterDefault"
	actionTreeFilterNoTools  = "treeFilterNoTools"
	actionTreeFilterUserOnly = "treeFilterUserOnly"
	actionTreeFilterLabeled  = "treeFilterLabeledOnly"
	actionTreeFilterAll      = "treeFilterAll"
	actionSessionToggleSort  = "sessionToggleSort"
	actionSessionToggleNamed = "sessionToggleNamed"
	actionSessionDelete      = "sessionDelete"
	actionSessionRename      = "sessionRename"
)

// overlayActionDefaults — дефолтные аккорды для оверлеев tree/session.
// Хранятся в Keybindings, чтобы user keybindings.json мог переопределить.
func overlayActionDefaults() map[string]string {
	return map[string]string{
		actionTreeFilterDefault:   "ctrl+d",
		actionTreeFilterNoTools:   "ctrl+t",
		actionTreeFilterUserOnly:  "ctrl+u",
		actionTreeFilterLabeled:   "ctrl+l",
		actionTreeFilterAll:       "ctrl+a",
		actionTreeFilterCycle:     "ctrl+o",
		actionTreeFilterCycleBack: "shift+ctrl+o",
		actionSessionToggleSort:   "ctrl+s",
		actionSessionToggleNamed:  "ctrl+n",
		actionSessionDelete:       "ctrl+d",
		actionSessionRename:       "ctrl+r",
	}
}

func defaultOverlayKeyMap() *KeyMap {
	return overlayKeyMapFrom(overlayActionDefaults())
}

func overlayKeyMapFrom(bindings map[string]string) *KeyMap {
	m := NewKeyMap()
	for action, keys := range bindings {
		for _, key := range splitBindingKeys(keys) {
			m.Bind(key, action)
		}
	}
	return m
}

// rebuildOverlayKeys объединяет дефолты оверлеев с переопределениями Keybindings пользователя.
func (m *Model) rebuildOverlayKeys() {
	merged := overlayActionDefaults()
	if m != nil {
		for action := range merged {
			if v, ok := m.keys.Bindings[action]; ok && v != "" {
				merged[action] = v
			}
		}
	}
	m.overlayKeys = overlayKeyMapFrom(merged)
}

func (m *Model) overlayKeyAction(key string) (string, bool) {
	if m.overlayKeys == nil {
		m.rebuildOverlayKeys()
	}
	// Удаление сессии важнее дефолтного фильтра дерева, когда оба на ctrl+d.
	if m.overlayMode == overlaySession && key == "ctrl+d" {
		return actionSessionDelete, true
	}
	if m.overlayMode == overlayTree && key == "ctrl+d" {
		return actionTreeFilterDefault, true
	}
	// Переименование в scoped-режиме всегда использует binding переименования сессии.
	if m.overlayMode == overlaySession && key == "ctrl+r" {
		if a, ok := m.overlayKeys.Lookup(key); ok && a == actionSessionRename {
			return a, true
		}
		return actionSessionRename, true
	}
	return m.overlayKeys.Lookup(key)
}

func (m *Model) dispatchOverlayKeyAction(action string) bool {
	switch action {
	case actionTreeFilterDefault:
		if m.overlayMode != overlayTree {
			return false
		}
		m.setTreeFilter("default")
		return true
	case actionTreeFilterNoTools:
		if m.overlayMode != overlayTree {
			return false
		}
		m.setTreeFilter("noTools")
		return true
	case actionTreeFilterUserOnly:
		if m.overlayMode != overlayTree {
			return false
		}
		m.setTreeFilter("userOnly")
		return true
	case actionTreeFilterLabeled:
		if m.overlayMode != overlayTree {
			return false
		}
		m.setTreeFilter("labeledOnly")
		return true
	case actionTreeFilterAll:
		if m.overlayMode != overlayTree {
			return false
		}
		m.setTreeFilter("all")
		return true
	case actionTreeFilterCycle:
		if m.overlayMode != overlayTree {
			return false
		}
		m.cycleTreeFilter()
		return true
	case actionTreeFilterCycleBack:
		if m.overlayMode != overlayTree {
			return false
		}
		m.cycleTreeFilterBack()
		return true
	case actionSessionToggleSort:
		if m.overlayMode != overlaySession {
			return false
		}
		m.sessionSortDesc = !m.sessionSortDesc
		m.sessionItems = m.sortSessionItems(m.sessionItems)
		m.overlayCursor = 0
		m.overlay = renderSessionOverlay(m.sessionItems, m.overlayCursor, m.sessionSortDesc, m.sessionNamedOnly)
		return true
	case actionSessionToggleNamed:
		if m.overlayMode != overlaySession {
			return false
		}
		m.sessionNamedOnly = !m.sessionNamedOnly
		m.openSessionOverlay()
		return true
	case actionSessionDelete:
		if m.overlayMode != overlaySession {
			return false
		}
		m.confirmDeleteSession()
		return true
	case actionSessionRename:
		if m.overlayMode != overlaySession {
			return false
		}
		if len(m.sessionItems) == 0 {
			return true
		}
		m.openSessionRenameOverlay()
		return true
	default:
		return false
	}
}
