package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// appRoot собирает дерево дифференциальных Component для интерактивного режима:
// Порядок слоёв: header → chat → pending → status → widgetsAbove → editor|selector → widgetsBelow → footer
type appRoot struct {
	m *Model
}

func (a *appRoot) Render(width int) []string {
	m := a.m
	m.width = width
	if width <= 0 {
		return []string{"loading…"}
	}

	m.syncEditorBorder()

	var out []string

	// Заголовок расширения над чатом (header → chat).
	if m.extHeader != "" {
		out = append(out, strings.Split(strings.TrimRight(m.extHeader, "\n"), "\n")...)
	}

	selector := m.isEditorSelector()
	if m.overlay != "" && !selector {
		olLines := strings.Split(strings.TrimRight(m.overlay, "\n"), "\n")
		olLines = ClampOverlayLines(olLines, m.overlayMaxHeightPct, m.height)
		chat := m.viewport.View()
		chatLines := strings.Split(strings.TrimRight(chat, "\n"), "\n")
		opts := OverlayOptions{MaxHeightPct: m.overlayMaxHeightPct}
		switch m.overlayAnchor {
		case overlayAnchorBottom:
			opts.Anchor = OverlayAnchorBottom
			out = append(out, CompositeOverlayLines(chatLines, olLines, opts, m.height)...)
		case overlayAnchorTop:
			opts.Anchor = OverlayAnchorTop
			out = append(out, CompositeOverlayLines(chatLines, olLines, opts, m.height)...)
		case overlayAnchorCenter:
			opts.Anchor = OverlayAnchorCenter
			out = append(out, CompositeOverlayLines(chatLines, olLines, opts, m.height)...)
		default:
			out = append(out, olLines...)
		}
		if m.overlayMode != overlayGrant {
			out = append(out, m.colors.muted().Render("(esc to close)"))
		}
	} else {
		chat := m.viewport.View()
		out = append(out, strings.Split(strings.TrimRight(chat, "\n"), "\n")...)
	}

	if pending := m.renderPendingStrip(width); pending != "" {
		out = append(out, "") // Отступ в одну строку
		out = append(out, strings.Split(pending, "\n")...)
	}
	if status := m.renderStatusStrip(width); status != "" {
		out = append(out, status)
	}
	// Индикатор работы в области статуса (не в футере).
	if m.busy && m.overlay == "" && m.extReplaceEditor == "" && !selector {
		label := m.extWorking
		if label == "" {
			hint := KeyDisplay(m.keys, actionInterrupt)
			if hint == "" {
				hint = "esc"
			}
			label = fmt.Sprintf("Working... (%s to interrupt)", hint)
		}
		m.loader.Label = label
		m.loader.Advance()
		if len(m.extWorkingFrames) > 0 {
			f := m.extWorkingFrames[(m.loader.Tick-1)%len(m.extWorkingFrames)]
			out = append(out, m.colors.header().Render(truncate(f+" "+label, width)))
		} else {
			out = append(out, m.loader.Render(width)...)
		}
	}
	if m.extAbove != "" {
		out = append(out, strings.Split(m.extAbove, "\n")...)
	}

	if selector {
		selW := width
		if selW > 80 {
			selW = 80
		}
		ol := m.overlay
		if m.overlayComp != nil {
			ol = stringsJoinOverlay(m.overlayComp.Render(selW - 4))
		}
		title, hints := m.selectorChrome()
		out = append(out, renderDynamicBorder(ol, title, hints, selW, m.colors)...)
		out = append(out, m.colors.muted().Render("(esc to close)"))
	} else if m.extReplaceEditor != "" {
		out = append(out, strings.Split(m.extReplaceEditor, "\n")...)
	} else {
		if panel := m.renderWorkflowPanel(); panel != "" {
			out = append(out, strings.Split(panel, "\n")...)
		}
		if popup := m.slashMenuPopup(); popup != "" {
			out = append(out, strings.Split(strings.TrimRight(popup, "\n"), "\n")...)
		} else if popup := m.autocompletePopup(); popup != "" {
			out = append(out, strings.Split(strings.TrimRight(popup, "\n"), "\n")...)
		}
		if popup := m.composerPopup(); popup != "" {
			out = append(out, strings.Split(strings.TrimRight(popup, "\n"), "\n")...)
		}
		if len(m.attachments) > 0 {
			chips := m.renderAttachmentChips(width)
			out = append(out, strings.Split(strings.TrimRight(chips, "\n"), "\n")...)
		}
		edLines := m.composer.ed.Render(width)
		out = append(out, edLines...)
	}

	if m.extBelow != "" {
		out = append(out, strings.Split(m.extBelow, "\n")...)
	}
	out = append(out, m.renderFooterBars(width)...)
	if m.errLine != "" {
		out = append(out, m.colors.err().Render(m.errLine))
	}

	if m.height > 0 {
		for len(out) < m.height {
			out = append(out, "")
		}
		if len(out) > m.height {
			out = out[len(out)-m.height:]
		}
	}
	return out
}

// isEditorSelector — режимы showSelector (заменяет редактор, чат остаётся).
func (m *Model) isEditorSelector() bool {
	switch m.overlayMode {
	case overlayTree, overlaySession, overlayModel, overlaySettings, overlayRename, overlayConfirm:
		return m.overlay != "" || m.overlayComp != nil
	default:
		return false
	}
}

func (m *Model) selectorChrome() (title, hints string) {
	switch m.overlayMode {
	case overlayTree:
		return "session tree", "↑/↓ enter navigate · F fork · s switch · f filter · esc"
	case overlaySession:
		return "sessions", "↑/↓ enter open · ctrl+r rename · ctrl+d delete · esc"
	case overlaySettings:
		return "settings", "↑/↓ ←/→ cycle · enter · esc"
	case overlayRename:
		return "rename session", "enter confirm · esc"
	case overlayModel:
		switch m.overlayList {
		case "theme":
			return "themes", "↑/↓ enter · esc"
		case "skill":
			return "skills", "↑/↓ enter · esc"
		case "prompt":
			return "prompts", "↑/↓ enter · esc"
		case "scoped":
			return "scoped models", "↑/↓ enter · esc"
		default:
			return "models", "↑/↓ enter · esc"
		}
	case overlayConfirm:
		return "confirm", "y confirm · n / esc cancel"
	default:
		return "selector", "esc to close"
	}
}

func (m *Model) syncEditorBorder() {
	if m.composer.ed == nil {
		return
	}
	st := m.svc.GetState()
	level := st.ThinkingLevel
	var ansi string
	switch level {
	case "low":
		ansi = colorSeq(m.colors.Muted)
	case "medium":
		ansi = colorSeq(m.colors.Accent)
	case "high", "xhigh", "max":
		ansi = colorSeq(m.colors.Tool)
	default:
		ansi = colorSeq(m.colors.Border)
	}
	m.composer.ed.SetBorderColor(ansi)
	m.composer.ed.SetBoxBorder(true)
}

func (m *Model) renderPendingStrip(width int) string {
	steer, follow := m.svc.QueueSnapshot()
	if len(steer) == 0 && len(follow) == 0 {
		return ""
	}
	var lines []string
	for _, s := range steer {
		lines = append(lines, m.colors.muted().Render(truncate("Steering: "+trimOneLine(s, width-12), width)))
	}
	for _, s := range follow {
		lines = append(lines, m.colors.muted().Render(truncate("Follow-up: "+trimOneLine(s, width-14), width)))
	}
	hint := KeyDisplay(m.keys, actionMessageDequeue)
	if hint == "" {
		hint = "alt+up"
	}
	lines = append(lines, m.colors.muted().Render(truncate(fmt.Sprintf("↳ %s to edit all queued messages", hint), width)))
	return strings.Join(lines, "\n")
}

func trimOneLine(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if n > 0 && len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

func (m *Model) renderStatusStrip(width int) string {
	msg := m.statusLine
	if msg == "" {
		msg = m.transientNotice
	}
	if msg == "" {
		return ""
	}
	// Префикс-спиннер для активных статусов (retry / compact / branch).
	spin := "◌"
	if m.retryInfo != nil || strings.Contains(strings.ToLower(msg), "compact") || strings.Contains(strings.ToLower(msg), "branch") {
		m.loader.Advance()
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		spin = frames[m.loader.Tick%len(frames)]
	}
	return m.colors.muted().Render(truncate(spin+" "+msg, width))
}

func (m *Model) renderFooterBars(width int) []string {
	st := m.svc.GetState()
	cwd := trimPath(m.cfg.Workspace, 40)
	if abs, err := filepath.Abs(m.cfg.Workspace); err == nil {
		cwd = trimPath(abs, 40)
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(cwd, home) {
		cwd = "~" + strings.TrimPrefix(cwd, home)
	}
	if m.gitBranch == "" {
		m.gitBranch = detectGitBranch(m.cfg.Workspace)
	}
	// cwd (ветка) • имя сессии
	pwd := cwd
	if m.gitBranch != "" {
		pwd = fmt.Sprintf("%s (%s)", cwd, m.gitBranch)
	}
	if st.SessionName != "" {
		pwd = pwd + " • " + st.SessionName
	}
	line1 := m.colors.muted().Render(truncate(pwd, width))

	tokenBits := []string{}
	if st.InputTokens > 0 || st.CacheRead > 0 {
		tokenBits = append(tokenBits, fmt.Sprintf("↑%s", compactNum(st.InputTokens)))
	}
	if st.OutputTokens > 0 {
		tokenBits = append(tokenBits, fmt.Sprintf("↓%s", compactNum(st.OutputTokens)))
	}
	if st.CacheRead > 0 {
		tokenBits = append(tokenBits, fmt.Sprintf("R%s", compactNum(st.CacheRead)))
	}
	if st.CacheWrite > 0 {
		tokenBits = append(tokenBits, fmt.Sprintf("W%s", compactNum(st.CacheWrite)))
	}
	if st.CacheHitRate > 0 {
		tokenBits = append(tokenBits, fmt.Sprintf("CH%.1f%%", float64(st.CacheHitRate)))
	}
	parts := []string{}
	if len(tokenBits) > 0 {
		parts = append(parts, strings.Join(tokenBits, " "))
	}
	usingSub := m.cfg != nil && m.cfg.Auth != nil && m.cfg.Auth.IsOAuth(st.Provider)
	if st.Cost != nil || usingSub {
		cost := 0.0
		if st.Cost != nil {
			cost = *st.Cost
		}
		costStr := fmt.Sprintf("$%.3f", cost)
		if usingSub {
			costStr += " (sub)"
		}
		parts = append(parts, costStr)
	}
	if st.ContextWindow > 0 {
		tok := st.ContextTokens
		auto := ""
		if st.AutoCompactionEnabled {
			auto = " (auto)"
		}
		var ctxDisplay string
		var pct float64
		if tok == 0 {
			ctxDisplay = fmt.Sprintf("?/%s%s", compactNum(st.ContextWindow), auto)
		} else {
			pct = float64(tok) * 100 / float64(st.ContextWindow)
			if pct > 100 {
				pct = 100
			}
			ctxDisplay = fmt.Sprintf("%.1f%%/%s%s", pct, compactNum(st.ContextWindow), auto)
		}
		colored := ctxDisplay
		if pct > 90 {
			colored = m.colors.err().Render(ctxDisplay)
		} else if pct > 70 {
			colored = colorSeq(warnColor(m.colors)) + ctxDisplay + reset
		}
		parts = append(parts, colored)
	}
	// Справа: (провайдер) модель • уровень thinking
	right := st.ModelName
	if st.Provider != "" {
		right = fmt.Sprintf("(%s) %s", st.Provider, st.ModelName)
	}
	level := st.ThinkingLevel
	if level == "" {
		level = "off"
	}
	right += " • " + level

	left := strings.Join(parts, "  ")
	gap := width - visibleLen(left) - visibleLen(right) - 1
	if gap < 1 {
		gap = 1
	}
	line2 := m.colors.muted().Render(truncate(left+strings.Repeat(" ", gap)+right, width))

	lines := []string{line1, line2}
	// Строка 3: статусы расширений (строка расширений в футере).
	if m.extFooter != "" {
		lines = append(lines, m.colors.muted().Render(truncate(m.extFooter, width)))
	} else if m.extWidget != "" {
		lines = append(lines, m.colors.muted().Render(truncate(m.extWidget, width)))
	}
	return lines
}

func warnColor(p palette) string {
	if p.Tokens != nil {
		if w := p.Tokens["warning"]; w != "" {
			return w
		}
	}
	return "214"
}

func detectGitBranch(cwd string) string {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func compactNum(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
