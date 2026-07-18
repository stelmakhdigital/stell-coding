package tui

import (
	"fmt"
	"strings"
	"time"

	"stell/agent/session"
)

type overlayMode int

const (
	overlayNone overlayMode = iota
	overlayTree
	overlayPicker
	overlayInlineAt
	overlayModel
	overlayGrant
	overlaySession
	overlayUI
	overlayRename
	overlaySettings
	overlayConfirm
	overlayMarkdownPreview
)

type treeItem struct {
	id        string
	parentID  string
	label     string
	typ       string
	timestamp string
	depth     int
	hasKids   bool
	isLeaf    bool
}

var treeFilterModes = []string{"default", "noTools", "userOnly", "labeledOnly", "all"}

func buildTreeItems(sess *session.Manager, filter, search string, folded map[string]bool) []treeItem {
	tree := sess.GetTree()
	byID := map[string]session.TreeNode{}
	children := map[string][]string{}
	for _, n := range tree {
		byID[n.ID] = n
		pid := ""
		if n.ParentID != nil {
			pid = *n.ParentID
		}
		children[pid] = append(children[pid], n.ID)
	}

	searchTokens := strings.Fields(strings.ToLower(search))
	matchesSearch := func(n session.TreeNode) bool {
		if len(searchTokens) == 0 {
			return true
		}
		hay := strings.ToLower(n.Label + " " + n.Type + " " + n.ID)
		for _, tok := range searchTokens {
			if !strings.Contains(hay, tok) {
				return false
			}
		}
		return true
	}
	passesFilter := func(n session.TreeNode) bool {
		switch filter {
		case "userOnly":
			return n.Type == "message" && strings.HasPrefix(n.Label, "user:")
		case "noTools":
			return n.Type != "bash" && !strings.HasPrefix(n.Label, "bashExecution:") && !strings.HasPrefix(n.Label, "toolResult:") && n.Type != "toolResult"
		case "labeledOnly":
			return n.Type == "label"
		case "all", "default", "":
			return true
		default:
			return true
		}
	}

	// Визуальный отступ: линейные цепочки single-child остаются плоскими; углубляем только на ветках.
	childIndentOf := func(indent int, justBranched bool, kidCount int) (childIndent int, childJustBranched bool) {
		multiple := kidCount > 1
		if multiple {
			return indent + 1, true
		}
		if justBranched && indent > 0 {
			return indent + 1, false
		}
		return indent, false
	}

	var out []treeItem
	var walk func(pid string, indent int, justBranched bool, ancestorFolded bool)
	walk = func(pid string, indent int, justBranched bool, ancestorFolded bool) {
		for _, id := range children[pid] {
			n := byID[id]
			if ancestorFolded {
				continue
			}
			kidIDs := children[id]
			nextIndent, nextBranched := childIndentOf(indent, justBranched, len(kidIDs))

			if !passesFilter(n) || !matchesSearch(n) {
				// Скрытый узел: обходим детей с тем же визуальным отступом (ближайший видимый предок).
				walk(id, indent, justBranched, false)
				continue
			}
			parent := ""
			if n.ParentID != nil {
				parent = *n.ParentID
			}
			label := n.Label
			if n.IsLeaf {
				label += " *"
			}
			if n.Type != "" && n.Type != "message" {
				label = "[" + n.Type + "] " + label
			}
			hasKids := len(n.Children) > 0
			out = append(out, treeItem{
				id: n.ID, parentID: parent, label: label, typ: n.Type,
				timestamp: n.Timestamp,
				depth: indent, hasKids: hasKids, isLeaf: n.IsLeaf,
			})
			isFolded := folded != nil && folded[id]
			walk(id, nextIndent, nextBranched, isFolded)
		}
	}

	roots := children[""]
	rootIndent := 0
	rootBranched := false
	if len(roots) > 1 {
		rootIndent = 1
		rootBranched = true
	}
	walk("", rootIndent, rootBranched, false)
	return out
}

func formatTreeTs(ts string) string {
	if ts == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
	}
	if err != nil {
		if len(ts) >= 19 {
			return ts[:19]
		}
		return ts
	}
	return t.Local().Format("15:04:05")
}

func renderTreeOverlay(items []treeItem, cursor int, filter, search string, folded map[string]bool, showTs bool) string {
	var b strings.Builder
	b.WriteString("session tree (↑/↓ enter fork · s switch · f filter · T timestamps · type to search · esc)\n")
	filt := filter
	if filt == "" {
		filt = "default"
	}
	fmt.Fprintf(&b, "  filter: %s", filt)
	if search != "" {
		fmt.Fprintf(&b, "  search: %q", search)
	}
	if showTs {
		b.WriteString("  timestamps: on")
	}
	b.WriteString("\n")
	if len(items) > 0 {
		fmt.Fprintf(&b, "  (%d/%d)\n", cursor+1, len(items))
	} else {
		b.WriteString("  (empty)\n")
	}
	for i, it := range items {
		prefix := "  "
		if i == cursor {
			prefix = "→ "
		}
		indent := strings.Repeat("│ ", it.depth)
		if it.depth > 0 {
			indent = strings.Repeat("  ", it.depth-1) + "├─ "
		}
		foldMark := ""
		if it.hasKids {
			if folded != nil && folded[it.id] {
				foldMark = "▸ "
			} else {
				foldMark = "▾ "
			}
		}
		b.WriteString(prefix)
		b.WriteString(indent)
		b.WriteString(foldMark)
		if showTs {
			if ft := formatTreeTs(it.timestamp); ft != "" {
				b.WriteString(ft)
				b.WriteString(" ")
			}
		}
		b.WriteString(it.label)
		b.WriteString("\n")
	}
	return b.String()
}

func renderPickerOverlay(files []string, cursor int) string {
	if len(files) == 0 {
		return "file picker: (no matches)\n(esc to close)"
	}
	var b strings.Builder
	b.WriteString("file picker (↑/↓, enter insert, esc close)\n")
	for i, f := range files {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		b.WriteString(prefix)
		b.WriteString(f)
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) openTreeOverlay() {
	if m.treeFolded == nil {
		m.treeFolded = map[string]bool{}
	}
	if m.treeFilter == "" {
		if m.cfg != nil {
			m.treeFilter = m.cfg.Settings.TreeFilterModeOrDefault()
		} else {
			m.treeFilter = "default"
		}
	}
	m.treeItems = buildTreeItems(m.svc.Sessions, m.treeFilter, m.treeSearch, m.treeFolded)
	m.svc.EmitSessionTree(m.ctx, len(m.svc.Sessions.Entries))
	m.pushOverlayFrame(overlayFrame{
		mode:       overlayTree,
		text:       renderTreeOverlay(m.treeItems, 0, m.treeFilter, m.treeSearch, m.treeFolded, m.treeShowTs),
		cursor:     0,
		treeItems:  append([]treeItem(nil), m.treeItems...),
		treeFilter: m.treeFilter,
	})
}

func (m *Model) openPickerOverlay(query string) {
	m.pushOverlayFrame(overlayFrame{
		mode:        overlayPicker,
		text:        renderPickerOverlay(m.filterFiles(query), 0),
		cursor:      0,
		pickerFiles: m.filterFiles(query),
	})
	m.pickerQuery = query
}

func (m *Model) handleConfirmOverlayKey(key string) bool {
	switch key {
	case "y", "Y", "enter":
		path := m.pendingDeleteSession
		m.pendingDeleteSession = ""
		// Сбрасываем confirm и устаревший кадр сессии, чтобы reopen был чистым.
		m.clearOverlayState()
		if path == "" {
			return true
		}
		if err := m.svc.DeleteSession(path); err != nil {
			m.addError(err.Error())
		} else {
			m.addInfo("deleted " + path)
			m.openSessionOverlay()
		}
		return true
	case "n", "N", "esc":
		m.pendingDeleteSession = ""
		if !m.popOverlayFrame() {
			m.closeOverlay()
		}
		return true
	default:
		return true
	}
}

func (m *Model) handleOverlayKey(key string) bool {
	if m.overlayMode == overlayConfirm {
		return m.handleConfirmOverlayKey(key)
	}
	// Inline @-picker: ключи обрабатывает tui.go (не материализуем полный overlay).
	if m.overlayMode == overlayInlineAt {
		return false
	}
	switch m.overlayMode {
	case overlayTree:
		if m.handleTreeOverlayKey(key) {
			return true
		}
	case overlaySession:
		if m.handleSessionOverlayKey(key) {
			return true
		}
	case overlayRename:
		return m.handleRenameOverlayKey(key)
	case overlayModel:
		if m.overlayList == "scoped" && m.handleScopedModelsKey(key) {
			return true
		}
		if sl, ok := m.overlayComp.(*SelectList); ok {
			switch key {
			case "up", "shift+tab", "k":
				sl.HandleInput("\x1b[A")
				m.overlayCursor = sl.Cursor
			case "down", "tab", "j":
				sl.HandleInput("\x1b[B")
				m.overlayCursor = sl.Cursor
			case "enter":
				sl.HandleInput("\r")
				if sl.Selected != "" {
					m.closeOverlay()
				}
				return true
			case "esc":
				if sl.Query != "" {
					sl.SetFilter("")
					m.overlayCursor = sl.Cursor
				} else {
					m.closeOverlay()
				}
				return true
			case "backspace":
				sl.HandleInput("\x7f")
				m.overlayCursor = sl.Cursor
			default:
				if len(key) == 1 {
					sl.HandleInput(key)
					m.overlayCursor = sl.Cursor
					m.syncOverlayFromComp()
					return true
				}
				return false
			}
			m.syncOverlayFromComp()
			return true
		}
	case overlayUI:
		if m.handleUIOverlayKey(key) {
			return true
		}
		return false
	case overlaySettings:
		return m.handleSettingsOverlayKey(key)
	case overlayGrant:
		switch key {
		case "y", "Y":
			m.respondGrant(true)
			return true
		case "n", "N", "esc":
			m.respondGrant(false)
			return true
		}
		return false
	case overlayMarkdownPreview:
		return m.handleMarkdownPreviewKey(key)
	}
	switch key {
	case "esc":
		if m.workflow != nil {
			m.cancelWorkflow()
			return true
		}
		m.closeOverlay()
		return true
	case "up", "shift+tab":
		if m.overlayCursor > 0 {
			m.overlayCursor--
		}
		m.refreshOverlay()
		return true
	case "down", "tab":
		max := m.overlayMax() - 1
		if m.overlayCursor < max {
			m.overlayCursor++
		}
		m.refreshOverlay()
		return true
	case "enter":
		m.handleOverlayEnter()
		return true
	default:
		return false
	}
}

func (m *Model) handleTreeOverlayKey(key string) bool {
	if action, ok := m.overlayKeyAction(key); ok {
		if m.dispatchOverlayKeyAction(action) {
			return true
		}
	}
	if key == "s" {
		m.switchTreeEntry()
		return true
	}
	if key == "shift+f" || key == "F" {
		m.forkTreeEntry()
		return true
	}
	if key == "shift+l" || key == "L" {
		m.editTreeLabel()
		return true
	}
	if key == "shift+t" || key == "T" {
		m.treeShowTs = !m.treeShowTs
		m.refreshOverlay()
		return true
	}
	if key == "f" {
		m.cycleTreeFilter()
		return true
	}
	if key == "backspace" || key == "ctrl+h" {
		if m.treeSearch != "" {
			r := []rune(m.treeSearch)
			m.treeSearch = string(r[:len(r)-1])
			m.rebuildTreeView()
			return true
		}
	}
	if key == "esc" && m.treeSearch != "" {
		m.treeSearch = ""
		m.rebuildTreeView()
		return true
	}
	// Печатаемые символы поиска (одна руна, без модификаторов).
	if len(key) == 1 {
		r := key[0]
		if r >= 32 && r < 127 && r != '/' {
			// '/' тоже явно начинает поиск
			m.treeSearch += key
			m.rebuildTreeView()
			return true
		}
	}
	if key == "/" {
		// фокус на поиск (уже через набор текста)
		return true
	}
	return false
}

func (m *Model) rebuildTreeView() {
	m.treeItems = buildTreeItems(m.svc.Sessions, m.treeFilter, m.treeSearch, m.treeFolded)
	if m.overlayCursor >= len(m.treeItems) {
		m.overlayCursor = len(m.treeItems) - 1
	}
	if m.overlayCursor < 0 {
		m.overlayCursor = 0
	}
	m.refreshOverlay()
}

func (m *Model) overlayMax() int {
	switch m.overlayMode {
	case overlayTree:
		return len(m.treeItems)
	case overlaySession:
		return len(m.sessionItems)
	case overlayPicker, overlayInlineAt:
		return len(m.pickerFiles)
	case overlayModel:
		return len(m.modelNames)
	case overlaySettings:
		if sl, ok := m.overlayComp.(*SettingsList); ok {
			return len(sl.Items)
		}
		return len(settingsItems)
	default:
		return 0
	}
}

func (m *Model) handleOverlayEnter() {
	switch m.overlayMode {
	case overlaySession:
		m.switchSessionFromOverlay()
	case overlayTree:
		if len(m.treeItems) == 0 {
			m.closeOverlay()
			return
		}
		// Enter — навигация (+ branch summary через SwitchToEntry).
		m.switchTreeEntry()
	case overlayPicker, overlayInlineAt:
		if len(m.pickerFiles) == 0 {
			m.closeOverlay()
			return
		}
		path := m.pickerFiles[m.overlayCursor]
		m.addAttachment(path)
		if m.overlayMode == overlayInlineAt {
			m.insertInlinePicker(path)
			return
		}
		m.closeOverlay()
	case overlayModel:
		if len(m.modelNames) == 0 {
			m.closeOverlay()
			return
		}
		name := m.modelNames[m.overlayCursor]
		switch m.overlayList {
		case "theme":
			m.cfg.Settings.Theme = name
			m.reloadTheme()
			m.addInfo("theme → " + name)
		case "skill":
			m.pendingCmd = m.submitWithText("/skill:"+name, false)
		case "prompt":
			text, err := m.svc.RenderPrompt(name, nil)
			if err != nil {
				m.addError(err.Error())
			} else {
				m.pendingCmd = m.submitWithText(text, false)
			}
		default:
			m.selectModel(name)
		}
		m.closeOverlay()
		m.syncViewport()
	case overlaySettings:
		if sl, ok := m.overlayComp.(*SettingsList); ok {
			sl.HandleInput("\r")
			m.syncOverlayFromComp()
			return
		}
		m.closeOverlay()
	default:
		m.closeOverlay()
	}
}

func (m *Model) switchTreeEntry() {
	if len(m.treeItems) == 0 {
		m.closeOverlay()
		return
	}
	id := m.treeItems[m.overlayCursor].id
	m.svc.EmitBeforeTree(m.ctx)
	if err := m.svc.SwitchToEntry(m.ctx, id); err != nil {
		m.addError(err.Error())
	} else {
		m.addInfo(fmt.Sprintf("switched to %s (branch summary may apply)", id))
		m.hydrateSession()
	}
	m.closeOverlay()
}

func (m *Model) forkTreeEntry() {
	if len(m.treeItems) == 0 {
		m.closeOverlay()
		return
	}
	id := m.treeItems[m.overlayCursor].id
	m.svc.EmitBeforeTree(m.ctx)
	leaf, err := m.svc.ForkSession(id)
	if err != nil {
		m.addError(err.Error())
	} else {
		m.addInfo(fmt.Sprintf("forked at %s → leaf %s", id, leaf))
		m.hydrateSession()
	}
	m.closeOverlay()
}

func (m *Model) refreshOverlay() {
	if m.overlayComp != nil {
		switch c := m.overlayComp.(type) {
		case *SelectList:
			c.Cursor = m.overlayCursor
		case *SettingsList:
			c.Cursor = m.overlayCursor
		}
		m.syncOverlayFromComp()
		m.syncViewport()
		return
	}
	switch m.overlayMode {
	case overlayTree:
		m.overlay = renderTreeOverlay(m.treeItems, m.overlayCursor, m.treeFilter, m.treeSearch, m.treeFolded, m.treeShowTs)
	case overlaySession:
		m.overlay = renderSessionOverlay(m.sessionItems, m.overlayCursor, m.sessionSortDesc, m.sessionNamedOnly)
	case overlayUI:
		m.overlay = m.renderUIOverlay()
	case overlayInlineAt:
		m.overlay = ""
	case overlayPicker:
		m.overlay = renderPickerOverlay(m.pickerFiles, m.overlayCursor)
	case overlayRename:
		m.overlay = m.renderRenameOverlay()
	case overlayModel:
		st := m.svc.GetState()
		switch m.overlayList {
		case "theme":
			m.overlay = renderThemeOverlay(m.modelNames, m.overlayCursor, m.cfg.Settings.Theme)
		case "skill", "prompt":
			m.overlay = renderListOverlay(m.overlayList+"s", m.modelNames, m.overlayCursor)
		case "scoped":
			m.overlay = m.renderScopedModelsOverlay()
		default:
			m.overlay = renderModelOverlay(m.modelNames, m.overlayCursor, st.ModelName)
		}
	case overlaySettings:
		m.overlay = renderSettingsOverlay(settingsItems, m.overlayCursor, m.cfg.Settings)
	}
	m.syncViewport()
}
