package tui

import (
	"strings"
)

type slashCommand struct {
	name string
	desc string
}

var baseSlashCommands = []slashCommand{
	{"/help", "Help"},
	{"/hotkeys", "Keyboard shortcuts"},
	{"/changelog", "Version notes"},
	{"/new", "New session"},
	{"/resume", "Session picker"},
	{"/tree", "Session tree"},
	{"/fork", "Fork at entry id"},
	{"/clone", "Clone session"},
	{"/export", "Export session"},
	{"/import", "Import session"},
	{"/copy", "Copy last assistant"},
	{"/preview", "Markdown preview"},
	{"/share", "Share as private gist"},
	{"/abort", "Cancel streaming run"},
	{"/commands", "List extension commands"},
	{"/compact", "Compact history"},
	{"/model", "Switch model"},
	{"/scoped-models", "Scoped models"},
	{"/theme", "Switch theme"},
	{"/themes", "List themes"},
	{"/session", "Session management"},
	{"/trust", "Trust workspace"},
	{"/reload", "Reload extensions"},
	{"/settings", "Settings"},
	{"/login", "OAuth login"},
	{"/logout", "Clear auth"},
	{"/skills", "List skills"},
	{"/prompts", "List prompts"},
	{"/quit", "Exit"},
}

type slashMenuState struct {
	filter string
	index  int
	items  []slashCommand
}

func wordAtEnd(s string) string {
	s = strings.TrimRight(s, " \n")
	i := strings.LastIndexAny(s, " \n")
	if i < 0 {
		return s
	}
	return s[i+1:]
}

func slashMenuScrollStart(index, total, maxVisible int) int {
	return popupScrollStart(index, total, maxVisible)
}

func (m *Model) updateSlashMenu() {
	word := wordAtEnd(m.composer.Value())
	if !strings.HasPrefix(word, "/") {
		m.slashMenu = nil
		return
	}
	q := strings.ToLower(word)
	all := append([]slashCommand(nil), baseSlashCommands...)
	for _, c := range m.svc.ExtensionCommands() {
		all = append(all, slashCommand{name: c.Name, desc: c.Description})
	}
	if m.svc.Catalog != nil && m.svc.Catalog.Prompts != nil {
		for _, e := range m.svc.Catalog.Prompts.List() {
			all = append(all, slashCommand{
				name: "/" + e.Name,
				desc: m.svc.Catalog.Prompts.SlashDescription(e.Name),
			})
		}
	}
	if m.svc.Catalog != nil && m.svc.Catalog.Skills != nil {
		for _, e := range m.svc.Catalog.Skills.List() {
			all = append(all, slashCommand{name: "/skill:" + e.Name, desc: e.Description})
		}
	}
	var items []slashCommand
	for _, c := range all {
		if strings.HasPrefix(strings.ToLower(c.name), q) {
			items = append(items, c)
		}
	}
	if len(items) == 0 {
		m.slashMenu = nil
		return
	}
	idx := 0
	if m.slashMenu != nil && m.slashMenu.filter == q {
		idx = m.slashMenu.index
		if idx >= len(items) {
			idx = len(items) - 1
		}
	}
	m.slashMenu = &slashMenuState{filter: q, index: idx, items: items}
}

func (m Model) slashMenuPopup() string {
	if m.slashMenu == nil || len(m.slashMenu.items) == 0 || m.overlay != "" {
		return ""
	}
	items := m.slashMenu.items
	labels := make([]string, len(items))
	for i, it := range items {
		labels[i] = it.name + "  " + it.desc
	}
	return renderScrollPopupStyled(
		"commands (↑/↓ tab complete)",
		labels,
		m.slashMenu.index,
		m.autocompleteMaxVisible(),
		colorSeq(m.colors.Muted),
		colorSeq(m.colors.Accent),
	)
}

func (m *Model) applySlashSelection() {
	if m.slashMenu == nil || len(m.slashMenu.items) == 0 {
		return
	}
	name := m.slashMenu.items[m.slashMenu.index].name
	text := m.composer.Value()
	word := wordAtEnd(text)
	if word == "" {
		m.composer.SetValue(name + " ")
	} else {
		m.composer.SetValue(strings.TrimSuffix(text, word) + name + " ")
	}
	m.slashMenu = nil
	m.resizeViewport()
}
