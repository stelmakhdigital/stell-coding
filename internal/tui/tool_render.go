package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/stelmakhdigital/stell-coding/internal/themes"
)

var runningHeartbeatRe = regexp.MustCompile(`(?m)^\[running \d+s / \d+s\]\n?`)

func stripRunningHeartbeat(s string) string {
	return strings.TrimSpace(runningHeartbeatRe.ReplaceAllString(s, ""))
}

func formatDurationSec(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func (c card) timingFooter(now time.Time, colors palette) string {
	if c.kind != cardTool && c.kind != cardBash {
		return ""
	}
	if c.startedAt.IsZero() {
		return ""
	}
	end := c.endedAt
	label := "Took"
	if c.status == cardStatusPending {
		label = "Elapsed"
		end = now
		if end.IsZero() {
			end = time.Now()
		}
	} else if end.IsZero() {
		end = c.startedAt
	}
	return colors.muted().Render(fmt.Sprintf("%s %s", label, formatDurationSec(end.Sub(c.startedAt))))
}

func toolArgString(args map[string]any, keys ...string) string {
	if args == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := args[k].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func toolTimeoutSec(args map[string]any) int {
	if args == nil {
		return 0
	}
	switch v := args["timeout"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}

func normalizeToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (m *Model) renderToolOrBashCard(idx int, c card, lay chatLayout) string {
	header, body := m.formatToolCardParts(c, lay.textW)
	header = padLinesLeft(header, lay.innerPad)
	if body != "" {
		body = padLinesLeft(body, lay.innerPad)
	}
	block := header
	if body != "" {
		if block != "" {
			block += "\n"
		}
		block += body
	}
	if footer := c.timingFooter(time.Now(), m.colors); footer != "" {
		if block != "" {
			block += "\n"
		}
		block += padLinesLeft(footer, lay.innerPad)
	}
	bg := m.colors.toolStatusBg(c.status)
	if c.status != cardStatusNone {
		block = paintLinesWithBg(block, bg, lay.contentW)
	}
	if img := m.renderCardImages(c, lay.contentW); img != "" {
		if block != "" {
			return block + "\n" + img
		}
		return img
	}
	return block
}

func (m *Model) formatToolCardParts(c card, width int) (header, body string) {
	name := normalizeToolName(c.toolName)
	if name == "" && c.body != "" {
		// Legacy-карточки только с body.
		return "", formatToolCard(m, c.body, width)
	}

	header = m.formatToolHeader(c, width)

	content := stripRunningHeartbeat(c.toolContent)
	if content == "" && strings.Contains(c.body, " → ") {
		_, rest, ok := strings.Cut(c.body, " → ")
		if ok {
			content = stripRunningHeartbeat(rest)
		}
	}

	switch name {
	case "read":
		if content == "" {
			return header, ""
		}
		return header, highlightToolContent(content, c.toolPath, m)
	case "write":
		preview := content
		if preview == "" {
			preview = c.toolContent
		}
		if preview == "" {
			return header, ""
		}
		return header, highlightToolContent(preview, c.toolPath, m)
	case "bash", "shell":
		fallthrough
	default:
		if c.kind == cardBash || name == "bash" || name == "shell" {
			out := content
			if out == "" {
				// User bash: body равен "$ cmd\noutput"
				lines := strings.SplitN(c.body, "\n", 2)
				if len(lines) > 1 {
					out = lines[1]
				}
			}
			out = stripRunningHeartbeat(out)
			if out == "" {
				return header, ""
			}
			return header, styleOutputLines(out, m.colors.toolOutput())
		}
		if looksLikeUnifiedDiff(content) {
			return header, renderUnifiedDiffHighlights(content, width, m.colors)
		}
		if content == "" {
			return header, ""
		}
		return header, styleOutputLines(content, m.colors.toolOutput())
	}
}

func styleOutputLines(text string, style ansiStyle) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = style.Render(line)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) formatToolHeader(c card, width int) string {
	name := c.toolName
	if name == "" {
		name = "tool"
	}
	accent := m.colors.tool().Render(name)
	path := strings.TrimSpace(c.toolPath)
	if c.kind == cardBash || normalizeToolName(name) == "bash" || normalizeToolName(name) == "shell" {
		prefix := "$ "
		if c.excludeBash {
			prefix = "$ !! "
		}
		cmd := path
		if cmd == "" {
			// Извлекаем из первой строки body.
			line := strings.SplitN(c.body, "\n", 2)[0]
			cmd = strings.TrimPrefix(strings.TrimPrefix(line, "$ !! "), "$ ")
		}
		h := m.colors.tool().Render(prefix) + m.colors.assistant().Render(cmd)
		if c.timeoutSec > 0 {
			h += m.colors.muted().Render(fmt.Sprintf(" (timeout %ds)", c.timeoutSec))
		}
		return wrapText(h, width)
	}
	if path != "" {
		return wrapText(accent+" "+m.colors.assistant().Render(path), width)
	}
	return wrapText(accent, width)
}

func highlightToolContent(content, path string, m *Model) string {
	if content == "" {
		return ""
	}
	md := m.activeTheme.MarkdownTheme()
	if md.Code == "" && m.cfg != nil {
		md = themes.DefaultTheme().MarkdownTheme()
	}
	_ = filepath.Ext(path) // reserved for language-aware highlighting
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, themes.HighlightCode(line, md))
	}
	return strings.Join(out, "\n")
}
