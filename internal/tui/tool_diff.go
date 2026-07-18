package tui

import (
	"strings"
)

// renderUnifiedDiffHighlights — подсветка unified diff токенами темы (toolDiff*).
func renderUnifiedDiffHighlights(diff string, width int, colors palette) string {
	if strings.TrimSpace(diff) == "" {
		return ""
	}
	var out []string
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			out = append(out, colors.diffAdded().Render(truncate(line, width)))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			out = append(out, colors.diffRemoved().Render(truncate(line, width)))
		case strings.HasPrefix(line, "@@"):
			out = append(out, colors.diffContext().Render(truncate(line, width)))
		default:
			out = append(out, colors.diffContext().Render(wrapText(line, width)))
		}
	}
	return strings.Join(out, "\n")
}
