package tui

import (
	"path/filepath"
	"strings"

	"github.com/mattn/go-runewidth"

	"stell/coding-agent/internal/agent"
)

type attachmentKind int

const (
	attachmentImage attachmentKind = iota
	attachmentFile
)

type composerAttachment struct {
	Path  string
	Label string
	Kind  attachmentKind
}

func newComposerAttachment(path string) composerAttachment {
	kind := attachmentFile
	if agent.IsImagePath(path) {
		kind = attachmentImage
	}
	label := filepath.Base(path)
	if label == ".stell-clipboard.png" || strings.HasSuffix(path, "/.stell-clipboard.png") {
		label = "Clipboard image"
	}
	return composerAttachment{Path: path, Label: label, Kind: kind}
}

func (m *Model) attachmentPaths() []string {
	paths := make([]string, len(m.attachments))
	for i, a := range m.attachments {
		paths[i] = a.Path
	}
	return paths
}

func (m *Model) addAttachment(path string) {
	for _, a := range m.attachments {
		if a.Path == path {
			return
		}
	}
	m.attachments = append(m.attachments, newComposerAttachment(path))
	m.attachmentFocus = len(m.attachments) - 1
	m.resizeViewport()
}

func (m *Model) removeAttachment(idx int) {
	if idx < 0 || idx >= len(m.attachments) {
		return
	}
	m.attachments = append(m.attachments[:idx], m.attachments[idx+1:]...)
	if len(m.attachments) == 0 {
		m.attachmentFocus = -1
		return
	}
	if m.attachmentFocus >= len(m.attachments) {
		m.attachmentFocus = len(m.attachments) - 1
	}
	m.resizeViewport()
}

func (m *Model) removeLastAttachment() {
	if len(m.attachments) == 0 {
		return
	}
	m.removeAttachment(len(m.attachments) - 1)
}

func (m *Model) composerInnerWidth() int {
	w := m.width - 6
	if w < 20 {
		w = 20
	}
	return w
}

func (m *Model) renderAttachmentChip(att composerAttachment, selected bool) string {
	icon := "📄"
	if att.Kind == attachmentImage {
		icon = "🖼"
	}
	label := att.Label
	maxLabel := 24
	if runewidth.StringWidth(label) > maxLabel {
		for len(label) > 0 && runewidth.StringWidth(label) > maxLabel-1 {
			label = label[:len(label)-1]
		}
		label += "…"
	}
	text := icon + " " + label + " ×"
	style := m.colors.chip()
	if selected {
		style = m.colors.chipSelected()
	}
	return style.Render(text)
}

func (m *Model) renderAttachmentChips(maxW int) string {
	if len(m.attachments) == 0 {
		return ""
	}
	const gap = 1
	var lines []string
	var line strings.Builder
	lineW := 0
	for i, att := range m.attachments {
		chip := m.renderAttachmentChip(att, i == m.attachmentFocus)
		chipW := styleWidth(chip)
		if lineW > 0 && lineW+gap+chipW > maxW {
			lines = append(lines, line.String())
			line.Reset()
			lineW = 0
		}
		if lineW > 0 {
			line.WriteByte(' ')
			lineW += gap
		}
		line.WriteString(chip)
		lineW += chipW
	}
	if lineW > 0 {
		lines = append(lines, line.String())
	}
	// Inline-превью сфокусированного image-вложения (showImages).
	if m.cfg != nil && m.cfg.Settings.ImagesEnabled() && m.attachmentFocus >= 0 && m.attachmentFocus < len(m.attachments) {
		att := m.attachments[m.attachmentFocus]
		if att.Kind == attachmentImage {
			lines = append(lines, m.renderInlineImage(att.Path, att.Label, maxW))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) handleAttachmentKeys(keyStr string) bool {
	if len(m.attachments) == 0 {
		return false
	}
	composerEmpty := m.composer.Value() == ""
	switch keyStr {
	case "backspace":
		if !composerEmpty {
			return false
		}
		if m.attachmentFocus >= 0 {
			m.removeAttachment(m.attachmentFocus)
		} else {
			m.removeLastAttachment()
		}
		return true
	case "delete":
		if !composerEmpty || m.attachmentFocus < 0 {
			return false
		}
		m.removeAttachment(m.attachmentFocus)
		return true
	case "left":
		if !composerEmpty {
			return false
		}
		if m.attachmentFocus <= 0 {
			m.attachmentFocus = 0
		} else {
			m.attachmentFocus--
		}
		return true
	case "right":
		if !composerEmpty {
			return false
		}
		if m.attachmentFocus < len(m.attachments)-1 {
			m.attachmentFocus++
		}
		return true
	}
	return false
}
