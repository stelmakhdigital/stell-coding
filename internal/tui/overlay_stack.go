package tui

// overlayAnchor задаёт, как app_root композитит оверлей и чат.
type overlayAnchor int

const (
	overlayAnchorFull overlayAnchor = iota // заменяет viewport чата
	overlayAnchorTop
	overlayAnchorCenter
	overlayAnchorBottom
)

// overlayFrame — один уровень стека оверлеев Model (push/pop и восстановление фокуса).
type overlayFrame struct {
	mode     overlayMode
	listKind string // model/theme/skill/prompt/scoped/settings
	comp     Component
	text     string
	cursor   int
	anchor       overlayAnchor
	maxHeightPct int // 1–100; 0 = без ограничения
	// Снимок состояния, восстанавливаемый при pop, когда кадр снова становится верхним.
	modelNames   []string
	treeItems    []treeItem
	pickerFiles  []string
	sessionItems []sessionItem
	uiOverlay    *uiOverlayState
	renameInput  string
	treeFilter   string
	overlayList  string
}

func (m *Model) pushOverlayFrame(f overlayFrame) {
	m.dismissPopups()
	// Сохраняем текущий верхний кадр в стек перед заменой (если есть).
	if m.overlayMode != overlayNone || m.overlay != "" || m.overlayComp != nil {
		m.overlayStack = append(m.overlayStack, m.snapshotOverlayFrame())
	}
	m.applyOverlayFrame(f)
}

func (m *Model) snapshotOverlayFrame() overlayFrame {
	return overlayFrame{
		mode:         m.overlayMode,
		listKind:     m.overlayList,
		comp:         m.overlayComp,
		text:         m.overlay,
		cursor:       m.overlayCursor,
		anchor:       m.overlayAnchor,
		maxHeightPct: m.overlayMaxHeightPct,
		modelNames:   append([]string(nil), m.modelNames...),
		treeItems:    append([]treeItem(nil), m.treeItems...),
		pickerFiles:  append([]string(nil), m.pickerFiles...),
		sessionItems: append([]sessionItem(nil), m.sessionItems...),
		uiOverlay:    m.uiOverlay,
		renameInput:  m.renameInput,
		treeFilter:   m.treeFilter,
		overlayList:  m.overlayList,
	}
}

func (m *Model) applyOverlayFrame(f overlayFrame) {
	m.overlayMode = f.mode
	m.overlayList = f.listKind
	if f.overlayList != "" {
		m.overlayList = f.overlayList
	}
	m.overlayComp = f.comp
	m.overlayCursor = f.cursor
	m.overlayAnchor = f.anchor
	m.overlayMaxHeightPct = f.maxHeightPct
	m.modelNames = f.modelNames
	m.treeItems = f.treeItems
	m.pickerFiles = f.pickerFiles
	m.sessionItems = f.sessionItems
	m.uiOverlay = f.uiOverlay
	m.renameInput = f.renameInput
	if f.treeFilter != "" || f.mode == overlayTree {
		m.treeFilter = f.treeFilter
	}
	if f.comp != nil {
		m.overlay = stringsJoinOverlay(f.comp.Render(m.width))
	} else {
		m.overlay = f.text
	}
}

func stringsJoinOverlay(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for i := 1; i < len(lines); i++ {
		out += "\n" + lines[i]
	}
	return out
}

func (m *Model) popOverlayFrame() bool {
	if len(m.overlayStack) == 0 {
		m.clearOverlayState()
		return false
	}
	top := m.overlayStack[len(m.overlayStack)-1]
	m.overlayStack = m.overlayStack[:len(m.overlayStack)-1]
	m.applyOverlayFrame(top)
	m.resizeViewport()
	return true
}

func (m *Model) clearOverlayState() {
	m.overlay = ""
	m.overlayComp = nil
	m.overlayMode = overlayNone
	m.overlayCursor = 0
	m.treeItems = nil
	m.pickerFiles = nil
	m.sessionItems = nil
	m.uiOverlay = nil
	m.modelNames = nil
	m.renameInput = ""
	m.overlayList = ""
	m.overlayMaxHeightPct = 0
	m.overlayStack = nil
	m.treeSearch = ""
	m.resizeViewport()
}

func (m *Model) closeOverlay() {
	if m.popOverlayFrame() {
		m.dismissPopups()
		m.syncViewport()
		return
	}
	m.clearOverlayState()
	m.dismissPopups()
}

func (m *Model) syncOverlayFromComp() {
	if m.overlayComp == nil {
		return
	}
	w := m.width
	if w <= 0 {
		w = 80
	}
	m.overlay = stringsJoinOverlay(m.overlayComp.Render(w))
}
