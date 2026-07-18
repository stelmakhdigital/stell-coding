package tui

import (
	"context"
	"strings"
)

// acKind — источник автодополнения (единый popup как у coding-agent AutocompleteProvider).
type acKind int

const (
	acSlash acKind = iota
	acFile
	acSkill
	acExt
)

type acItem struct {
	label string
	value string
	desc  string
	kind  acKind
}

type acState struct {
	query string
	index int
	items []acItem
	kind  acKind
}

func (m *Model) updateAutocomplete() {
	text := m.composer.Value()
	if q, ok := atQueryAtCursor(text); ok {
		files := m.filterFiles(q)
		items := make([]acItem, 0, len(files))
		for _, f := range files {
			items = append(items, acItem{label: f, value: f, kind: acFile})
		}
		if len(items) == 0 {
			m.autocomplete = nil
			if m.overlayMode == overlayInlineAt {
				m.closeOverlay()
			}
			return
		}
		idx := 0
		if m.autocomplete != nil && m.autocomplete.kind == acFile && m.autocomplete.query == q {
			idx = m.autocomplete.index
			if idx >= len(items) {
				idx = len(items) - 1
			}
		}
		m.autocomplete = &acState{query: q, index: idx, items: items, kind: acFile}
		m.slashMenu = nil
		m.overlayMode = overlayInlineAt
		m.pickerFiles = files
		m.pickerQuery = q
		m.overlayCursor = idx
		m.overlay = ""
		return
	}

	word := wordAtEnd(text)
	if strings.HasPrefix(word, "/") {
		q := strings.ToLower(word)
		all := m.collectAutocompleteSlash()
		var items []acItem
		for _, it := range all {
			if strings.HasPrefix(strings.ToLower(it.value), q) {
				items = append(items, it)
			}
		}
		if len(items) == 0 {
			m.autocomplete = nil
			m.slashMenu = nil
			return
		}
		idx := 0
		if m.autocomplete != nil && m.autocomplete.kind == acSlash && m.autocomplete.query == q {
			idx = m.autocomplete.index
			if idx >= len(items) {
				idx = len(items) - 1
			}
		}
		m.autocomplete = &acState{query: q, index: idx, items: items, kind: acSlash}
		// Держим slashMenu синхронным с существующими путями submit.
		smItems := make([]slashCommand, len(items))
		for i, it := range items {
			smItems[i] = slashCommand{name: it.value, desc: it.desc}
		}
		m.slashMenu = &slashMenuState{filter: q, index: idx, items: smItems}
		return
	}

	m.autocomplete = nil
	m.slashMenu = nil
	if m.svc.Extensions != nil && strings.TrimSpace(text) != "" {
		if extItems := m.svc.Extensions.QueryAutocomplete(context.Background(), text); len(extItems) > 0 {
			items := make([]acItem, 0, len(extItems))
			for _, s := range extItems {
				label := s["label"]
				if label == "" {
					label = s["value"]
				}
				items = append(items, acItem{label: label, value: s["value"], desc: s["desc"], kind: acExt})
			}
			m.autocomplete = &acState{query: text, items: items, kind: acExt}
			return
		}
	}
	if m.overlayMode == overlayInlineAt {
		m.closeOverlay()
	}
}

func (m *Model) collectAutocompleteSlash() []acItem {
	var all []acItem
	for _, c := range baseSlashCommands {
		all = append(all, acItem{label: c.name, value: c.name, desc: c.desc, kind: acSlash})
	}
	for _, c := range m.svc.ExtensionCommands() {
		all = append(all, acItem{label: c.Name, value: c.Name, desc: c.Description, kind: acSlash})
	}
	if m.svc.Catalog != nil && m.svc.Catalog.Prompts != nil {
		for _, e := range m.svc.Catalog.Prompts.List() {
			all = append(all, acItem{
				label: "/" + e.Name,
				value: "/" + e.Name,
				desc:  m.svc.Catalog.Prompts.SlashDescription(e.Name),
				kind:  acSlash,
			})
		}
	}
	if m.svc.Catalog != nil && m.svc.Catalog.Skills != nil {
		for _, e := range m.svc.Catalog.Skills.List() {
			all = append(all, acItem{
				label: "/skill:" + e.Name,
				value: "/skill:" + e.Name,
				desc:  e.Description,
				kind:  acSkill,
			})
		}
	}
	return all
}

func (m Model) autocompletePopup() string {
	if m.autocomplete == nil || len(m.autocomplete.items) == 0 || m.overlay != "" {
		return ""
	}
	// acFile рендерит composerPopup (@ picker) — не дублируем.
	if m.autocomplete.kind == acFile {
		return ""
	}
	title := "complete (↑/↓ tab)"
	switch m.autocomplete.kind {
	case acSlash, acSkill:
		title = "commands (↑/↓ tab complete)"
	}
	labels := make([]string, len(m.autocomplete.items))
	for i, it := range m.autocomplete.items {
		if it.desc != "" {
			labels[i] = it.label + "  " + it.desc
		} else {
			labels[i] = it.label
		}
	}
	return renderScrollPopupStyled(
		title,
		labels,
		m.autocomplete.index,
		m.autocompleteMaxVisible(),
		colorSeq(m.colors.Muted),
		colorSeq(m.colors.Accent),
	)
}

func (m *Model) applyAutocomplete() {
	if m.autocomplete == nil || len(m.autocomplete.items) == 0 {
		return
	}
	it := m.autocomplete.items[m.autocomplete.index]
	switch it.kind {
	case acFile:
		m.insertInlinePicker(it.value)
	case acSlash, acSkill, acExt:
		if it.kind == acExt {
			m.composer.SetValue(it.value)
		} else {
			m.applySlashSelection()
		}
	}
	m.autocomplete = nil
}

func (m Model) autocompleteMaxVisible() int {
	if m.cfg != nil {
		return m.cfg.Settings.AutocompleteMaxVisibleOrDefault()
	}
	return popupMaxRows
}
