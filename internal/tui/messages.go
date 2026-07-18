package tui

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/stelmakhdigital/stell-coding/internal/skills"
	"github.com/stelmakhdigital/stell-coding/internal/themes"
)

func (m *Model) renderCardBody(idx int, c card, lay chatLayout) string {
	switch c.kind {
	case cardUser:
		block := m.renderColoredMessageBlock(c.body, m.userMessageBg(), lay)
		if img := m.renderCardImages(c, lay.contentW); img != "" {
			if block != "" {
				return block + "\n" + img
			}
			return img
		}
		return block
	case cardSkill:
		text := c.skillBody
		if c.userTail != "" {
			if text != "" {
				text += "\n\n"
			}
			text += c.userTail
		}
		if text == "" {
			text = fmt.Sprintf("skill %s", c.skillName)
		} else {
			text = fmt.Sprintf("skill %s\n\n%s", c.skillName, text)
		}
		return m.renderColoredMessageBlock(text, m.userMessageBg(), lay)
	case cardAssistant:
		md := NewMarkdown(c.body, m.markdownTheme())
		md.HighlightCode = func(line string) string {
			return themes.HighlightCode(line, m.activeTheme.MarkdownTheme())
		}
		block := joinLines(md.Render(lay.textW))
		if img := m.renderCardImages(c, lay.textW); img != "" {
			if block != "" {
				return block + "\n" + img
			}
			return img
		}
		return block
	case cardThinking:
		prefix := m.colors.thinking().Render("thinking:")
		md := NewMarkdown(c.body, m.markdownTheme())
		md.HighlightCode = func(line string) string {
			return themes.HighlightCode(line, m.activeTheme.MarkdownTheme())
		}
		block := styleThinkingLines(joinLines(md.Render(lay.textW)), m.colors)
		if block == "" {
			return prefix
		}
		return prefix + "\n" + block
	case cardTool, cardBash:
		return m.renderToolOrBashCard(idx, c, lay)
	case cardInfo:
		return m.colors.muted().Render(wrapText("· "+c.body, lay.textW))
	case cardWarning:
		return formatWarningCard(m, c.skillName, c.body, lay.textW)
	case cardError:
		return m.colors.err().Render(wrapText("! "+c.body, lay.textW))
	default:
		return wrapText(c.body, lay.textW)
	}
}

func formatToolCard(m *Model, body string, width int) string {
	if looksLikeUnifiedDiff(body) {
		return renderUnifiedDiffHighlights(body, width, m.colors)
	}
	// Подсветка имени инструмента и пути: "read: path" или "bash: cmd"
	name, rest, ok := strings.Cut(body, ":")
	if !ok {
		name, rest, ok = strings.Cut(body, " →")
		if ok {
			line := m.colors.tool().Render(strings.TrimSpace(name)) + m.colors.muted().Render(" →") + wrapText(rest, width)
			return wrapText(line, width)
		}
		return m.colors.tool().Render(wrapText(body, width))
	}
	name = strings.TrimSpace(name)
	rest = strings.TrimSpace(rest)
	accent := m.colors.tool().Render(name)
	path := m.colors.assistant().Render(rest)
	return wrapText(accent+" "+path, width)
}

func looksLikeUnifiedDiff(body string) bool {
	return strings.Contains(body, "\n@@") || strings.HasPrefix(strings.TrimSpace(body), "@@") ||
		(strings.Contains(body, "\n+") && strings.Contains(body, "\n-") && strings.Count(body, "\n") > 2)
}

func cardFromUserContent(content string) card {
	if strings.HasPrefix(content, "[system steer]") {
		body := strings.TrimSpace(strings.TrimPrefix(content, "[system steer]"))
		return card{kind: cardInfo, body: body}
	}
	if sb, ok := skills.ParseSkillBlock(content); ok {
		return card{
			kind:      cardSkill,
			skillName: sb.Name,
			skillBody: sb.Content,
			userTail:  sb.UserMessage,
		}
	}
	return card{kind: cardUser, body: content}
}

func (m *Model) renderCardImages(c card, width int) string {
	if len(c.images) == 0 || m.cfg == nil || !m.cfg.Settings.ImagesEnabled() {
		return ""
	}
	caps := DetectCapabilities()
	var parts []string
	for i, img := range c.images {
		data, err := base64.StdEncoding.DecodeString(img.Data)
		if err != nil || len(data) == 0 {
			// Иногда base64 без padding — пробуем RawStd.
			data, err = base64.RawStdEncoding.DecodeString(img.Data)
		}
		alt := fmt.Sprintf("image %d", i+1)
		if err != nil || len(data) == 0 {
			parts = append(parts, ImageStub(minInt(width, 40), 3, alt))
			continue
		}
		if caps.Images == ImageNone {
			parts = append(parts, ImageStub(minInt(width, 40), 3, alt))
			continue
		}
		cells := m.cfg.Settings.ImageWidthCells
		if cells <= 0 || m.cfg.Settings.AutoResizeImagesEnabled() {
			cells = minInt(width/2, 40)
			if cells < 8 {
				cells = 8
			}
		}
		mime := img.MimeType
		if mime == "" {
			mime = "image/png"
		}
		enc := EncodeTerminalImage(caps.Images, mime, data, ImageRenderOptions{
			MaxWidthCells: cells,
			ImageID:       10 + i,
		})
		if enc == "" {
			parts = append(parts, ImageStub(minInt(width, 40), 3, alt))
			continue
		}
		parts = append(parts, enc+"\n"+m.colors.muted().Render("["+alt+"]"))
	}
	return strings.Join(parts, "\n")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
