package tui

import (
	"regexp"
	"strconv"
)

var reCellSize = regexp.MustCompile(`^\x1b\[6;(\d+);(\d+)t`)

// handleCellSizeReply обрабатывает ответы CSI 6;height;width t из stdin.
func handleCellSizeReply(data string, m *Model, ui *TUI) bool {
	sm := reCellSize.FindStringSubmatch(data)
	if sm == nil {
		return false
	}
	h, _ := strconv.Atoi(sm[1])
	w, _ := strconv.Atoi(sm[2])
	if w > 0 && h > 0 {
		ui.SetCellDimensions(w, h)
		if m != nil {
			m.cellW, m.cellH = w, h
		}
	}
	return true
}
