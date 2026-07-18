package tui

import "strings"

func (m *Model) headerLines() int {
	// Startup и контекст внутри viewport чата — без плотного верхнего заголовка.
	return 0
}

func (m *Model) footerLines() int {
	n := 2 // футер из 2 строк (cwd/ветка · tokens/cost/model)
	if m.composer.ed != nil {
		n += len(m.composer.ed.Render(max(1, m.width)))
	} else {
		n += m.composer.Height() + 2
	}
	if len(m.attachments) > 0 {
		chips := m.renderAttachmentChips(m.composerInnerWidth())
		n += strings.Count(chips, "\n") + 1
	}
	if panel := m.renderWorkflowPanel(); panel != "" {
		n += strings.Count(panel, "\n") + 1
	}
	if m.overlayMode == overlayInlineAt && len(m.pickerFiles) > 0 {
		n += scrollPopupLineCount(len(m.pickerFiles), m.autocompleteMaxVisible())
	}
	if m.slashMenu != nil && len(m.slashMenu.items) > 0 && m.overlay == "" {
		n += scrollPopupLineCount(len(m.slashMenu.items), m.autocompleteMaxVisible())
	}
	if m.busy {
		n++ // индикатор работы
	}
	steer, follow := m.svc.QueueSnapshot()
	if len(steer)+len(follow) > 0 {
		n++
	}
	if m.extAbove != "" {
		n += strings.Count(m.extAbove, "\n") + 1
	}
	if m.extBelow != "" {
		n += strings.Count(m.extBelow, "\n") + 1
	}
	if m.errLine != "" {
		n++
	}
	return n
}

func (m *Model) viewportHeight() int {
	if m.height <= 0 {
		return 4
	}
	h := m.height - m.headerLines() - m.footerLines()
	if h < 4 {
		h = 4
	}
	return h
}

func (m *Model) resizeViewport() {
	if m.width <= 0 {
		return
	}
	m.resizeComposer()
	m.viewport.Width = m.width
	m.viewport.Height = m.viewportHeight()
}
