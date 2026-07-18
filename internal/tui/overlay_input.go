package tui

func (m *Model) overlayActive() bool {
	return m.overlayMode != overlayNone || m.overlay != "" || m.overlayComp != nil
}

func (m *Model) dismissPopups() {
	m.slashMenu = nil
	m.autocomplete = nil
}

func (m *Model) popupsActive() bool {
	return m.slashMenu != nil || m.autocomplete != nil
}
