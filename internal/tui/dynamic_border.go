package tui

import "strings"

// renderDynamicBorder оборачивает контент в рамку DynamicBorder.
func renderDynamicBorder(content, title, hints string, width int, colors palette) []string {
	if width < 10 {
		width = 10
	}
	innerW := width - 2
	var body []string
	if title != "" {
		body = append(body, colors.header().Render(truncate(title, innerW)))
	}
	if hints != "" {
		body = append(body, colors.muted().Render(truncate(hints, innerW)))
	}
	if content != "" {
		for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
			// Пропускаем дубли заголовка, часто вшитые в текст оверлея.
			if title != "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), strings.ToLower(title)) {
				continue
			}
			body = append(body, truncate(line, innerW))
		}
	}
	if len(body) == 0 {
		body = []string{""}
	}
	top := "┌" + strings.Repeat("─", innerW) + "┐"
	bot := "└" + strings.Repeat("─", innerW) + "┘"
	borderANSI := colorSeq(colors.Border)
	out := make([]string, 0, len(body)+2)
	out = append(out, borderANSI+top+"\x1b[0m")
	for _, line := range body {
		pad := innerW - visibleLen(line)
		if pad < 0 {
			pad = 0
			line = truncate(line, innerW)
		}
		out = append(out, borderANSI+"│\x1b[0m"+line+strings.Repeat(" ", pad)+borderANSI+"│\x1b[0m")
	}
	out = append(out, borderANSI+bot+"\x1b[0m")
	return out
}
