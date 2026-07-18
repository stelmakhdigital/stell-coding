package tui

import "fmt"

func (m *Model) openSessionRenameOverlay() {
	m.renameInput = m.sessionItems[m.overlayCursor].label
	m.overlayMode = overlayRename
	m.overlay = m.renderRenameOverlay()
}

func (m *Model) renderRenameOverlay() string {
	return fmt.Sprintf("rename session (enter save, esc cancel)\n> %s", m.renameInput)
}

func (m *Model) handleRenameOverlayKey(key string) bool {
	switch key {
	case "esc":
		m.overlayMode = overlaySession
		m.overlay = renderSessionOverlay(m.sessionItems, m.overlayCursor, m.sessionSortDesc, m.sessionNamedOnly)
		return true
	case "enter":
		name := m.renameInput
		m.svc.SetSessionName(name)
		m.addInfo("session renamed → " + name)
		m.openSessionOverlay()
		return true
	case "backspace":
		if len(m.renameInput) > 0 {
			m.renameInput = m.renameInput[:len(m.renameInput)-1]
			m.overlay = m.renderRenameOverlay()
		}
		return true
	default:
		if len(key) == 1 {
			m.renameInput += key
			m.overlay = m.renderRenameOverlay()
		}
		return true
	}
}
