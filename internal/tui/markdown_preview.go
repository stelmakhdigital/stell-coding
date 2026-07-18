package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/stelmakhdigital/stell-coding/internal/themes"
)

// lastAssistantMarkdown возвращает тело последней assistant-карточки (или текст сессии).
func (m *Model) lastAssistantMarkdown() string {
	if m.svc != nil {
		if text := m.svc.GetLastAssistantText(); text != "" {
			return text
		}
	}
	for i := len(m.lines) - 1; i >= 0; i-- {
		if m.lines[i].kind == cardAssistant && strings.TrimSpace(m.lines[i].body) != "" {
			return m.lines[i].body
		}
	}
	return ""
}

func (m *Model) openMarkdownPreview() {
	src := m.lastAssistantMarkdown()
	if src == "" {
		m.addInfo("no assistant message to preview")
		return
	}
	md := NewMarkdown(src, m.markdownTheme())
	md.HighlightCode = func(line string) string {
		return themes.HighlightCode(line, m.activeTheme.MarkdownTheme())
	}
	w := m.width
	if w <= 0 {
		w = 80
	}
	m.previewLines = md.Render(w)
	m.previewScroll = 0
	m.pushOverlayFrame(overlayFrame{
		mode:         overlayMarkdownPreview,
		text:         m.renderMarkdownPreview(),
		anchor:       overlayAnchorFull,
		maxHeightPct: 100,
	})
}

func (m *Model) renderMarkdownPreview() string {
	bodyH := m.height - 4
	if bodyH < 3 {
		bodyH = 3
	}
	lines := m.previewLines
	total := len(lines)
	if m.previewScroll < 0 {
		m.previewScroll = 0
	}
	maxScroll := total - bodyH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.previewScroll > maxScroll {
		m.previewScroll = maxScroll
	}
	end := m.previewScroll + bodyH
	if end > total {
		end = total
	}
	slice := lines[m.previewScroll:end]
	pos := 0
	if total > 0 {
		pos = m.previewScroll + 1
	}
	title := m.colors.muted().Render(fmt.Sprintf(
		"Markdown preview  (%d/%d)  pgup/pgdn · esc close · e external",
		pos, total,
	))
	var b strings.Builder
	b.WriteString(title)
	b.WriteByte('\n')
	b.WriteString(strings.Join(slice, "\n"))
	return b.String()
}

func (m *Model) handleMarkdownPreviewKey(key string) bool {
	bodyH := m.height - 4
	if bodyH < 3 {
		bodyH = 3
	}
	switch key {
	case "esc":
		m.closeOverlay()
		return true
	case "up", "k":
		m.previewScroll--
	case "down", "j":
		m.previewScroll++
	case "pgup", "ctrl+u":
		m.previewScroll -= bodyH
	case "pgdown", "ctrl+d":
		m.previewScroll += bodyH
	case "home", "g":
		m.previewScroll = 0
	case "end", "G":
		m.previewScroll = len(m.previewLines)
	case "e", "E":
		m.closeOverlay()
		m.pendingCmd = m.openMarkdownPagerCmd()
		return true
	default:
		return false
	}
	m.overlay = m.renderMarkdownPreview()
	return true
}

func (m *Model) openMarkdownPagerCmd() Cmd {
	src := m.lastAssistantMarkdown()
	if src == "" {
		m.addInfo("no assistant message to preview")
		return nil
	}
	w := m.width
	if w <= 0 {
		w = 80
	}
	md := NewMarkdown(src, m.markdownTheme())
	md.HighlightCode = func(line string) string {
		return themes.HighlightCode(line, m.activeTheme.MarkdownTheme())
	}
	rendered := strings.Join(md.Render(w), "\n")
	pager := "less -R"
	if m.cfg != nil {
		pager = m.cfg.Settings.MarkdownPagerCommand()
	}
	return runExternalPager(pager, rendered)
}

func runExternalPager(pagerCmd, content string) Cmd {
	return func() Msg {
		tmp, err := os.CreateTemp("", "stell-md-*.txt")
		if err != nil {
			return errMsg{err: err}
		}
		path := tmp.Name()
		if _, err := tmp.WriteString(content); err != nil {
			_ = tmp.Close()
			_ = os.Remove(path)
			return errMsg{err: err}
		}
		_ = tmp.Close()
		defer os.Remove(path)

		if rawModeRestore != nil {
			rawModeRestore()
		}
		parts := strings.Fields(pagerCmd)
		if len(parts) == 0 {
			parts = []string{"less", "-R"}
		}
		cmd := exec.Command(parts[0], append(parts[1:], path)...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()

		restore, err := EnableRawMode()
		if err == nil {
			setRawModeRestore(restore)
		}
		w, h, _ := TermSize()
		return WindowSizeMsg{Width: w, Height: h}
	}
}
