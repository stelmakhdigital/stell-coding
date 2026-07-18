package tui

import (
	"strings"
)

func atQueryAtCursor(text string) (query string, ok bool) {
	at := strings.LastIndex(text, "@")
	if at < 0 {
		return "", false
	}
	if at > 0 {
		prev := text[at-1]
		if prev != ' ' && prev != '\n' && prev != '\t' {
			return "", false
		}
	}
	frag := text[at+1:]
	if strings.ContainsAny(frag, " \n\t") {
		return "", false
	}
	return frag, true
}

func (m *Model) updateInlinePicker() {
	text := m.composer.Value()
	q, ok := atQueryAtCursor(text)
	if !ok {
		if m.overlayMode == overlayInlineAt {
			m.closeOverlay()
		}
		return
	}
	files := m.filterFiles(q)
	m.overlayMode = overlayInlineAt
	m.pickerQuery = q
	m.pickerFiles = files
	m.overlayCursor = 0
	m.overlay = ""
}

func (m *Model) insertInlinePicker(path string) {
	text := m.composer.Value()
	at := strings.LastIndex(text, "@")
	if at < 0 {
		m.closeOverlay()
		return
	}
	m.addAttachment(path)
	newVal := strings.TrimRight(text[:at], " ")
	if newVal != "" && !strings.HasSuffix(newVal, " ") {
		newVal += " "
	}
	m.composer.SetValue(newVal)
	m.closeOverlay()
}
