package tui

func (m *Model) cycleTreeFilter() {
	cur := m.treeFilter
	if cur == "" {
		cur = "default"
	}
	idx := 0
	for i, f := range treeFilterModes {
		if f == cur {
			idx = i
			break
		}
	}
	m.treeFilter = treeFilterModes[(idx+1)%len(treeFilterModes)]
	m.overlayCursor = 0
	m.rebuildTreeView()
}

func (m *Model) cycleTreeFilterBack() {
	cur := m.treeFilter
	if cur == "" {
		cur = "default"
	}
	idx := 0
	for i, f := range treeFilterModes {
		if f == cur {
			idx = i
			break
		}
	}
	m.treeFilter = treeFilterModes[(idx-1+len(treeFilterModes))%len(treeFilterModes)]
	m.overlayCursor = 0
	m.rebuildTreeView()
}

func (m *Model) setTreeFilter(filter string) {
	m.treeFilter = filter
	m.overlayCursor = 0
	m.rebuildTreeView()
}

func (m *Model) editTreeLabel() {
	if len(m.treeItems) == 0 {
		return
	}
	id := m.treeItems[m.overlayCursor].id
	label := m.treeItems[m.overlayCursor].label
	if _, err := m.svc.Sessions.AppendLabel(label + " @ " + id); err != nil {
		m.addError(err.Error())
		return
	}
	if m.svc.SessPath != "" {
		_ = m.svc.Sessions.Save(m.svc.SessPath)
	}
	m.addInfo("label added for " + id)
	m.rebuildTreeView()
}

func (m *Model) treeFoldOrUp() {
	if len(m.treeItems) == 0 {
		return
	}
	if m.treeFolded == nil {
		m.treeFolded = map[string]bool{}
	}
	it := m.treeItems[m.overlayCursor]
	if it.hasKids && !m.treeFolded[it.id] {
		m.treeFolded[it.id] = true
		m.rebuildTreeView()
		return
	}
	if m.overlayCursor > 0 {
		m.overlayCursor--
		m.refreshOverlay()
	}
}

func (m *Model) treeUnfoldOrDown() {
	if len(m.treeItems) == 0 {
		return
	}
	if m.treeFolded == nil {
		m.treeFolded = map[string]bool{}
	}
	it := m.treeItems[m.overlayCursor]
	if it.hasKids && m.treeFolded[it.id] {
		delete(m.treeFolded, it.id)
		m.rebuildTreeView()
		return
	}
	if m.overlayCursor < len(m.treeItems)-1 {
		m.overlayCursor++
		m.refreshOverlay()
	}
}
