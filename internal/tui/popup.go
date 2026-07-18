package tui

import "strings"

const popupMaxRows = 9

func popupScrollStart(index, total, maxVisible int) int {
	if total <= maxVisible {
		return 0
	}
	start := index - maxVisible/2
	if start < 0 {
		return 0
	}
	if start+maxVisible > total {
		return total - maxVisible
	}
	return start
}

func scrollPopupLineCount(total, maxVisible int) int {
	if total == 0 {
		return 0
	}
	if maxVisible <= 0 {
		maxVisible = popupMaxRows
	}
	return 1 + 2 + maxVisible
}

func renderScrollPopup(title string, labels []string, cursor, maxVisible int) string {
	return renderScrollPopupStyled(title, labels, cursor, maxVisible, "", "")
}

// renderScrollPopupStyled рисует каждую строку своим SGR (безопасно для DiffEngine).
// mutedSeq оборачивает title / ellipsis / невыбранные строки; accentSeq — строку курсора.
func renderScrollPopupStyled(title string, labels []string, cursor, maxVisible int, mutedSeq, accentSeq string) string {
	total := len(labels)
	if total == 0 {
		return ""
	}
	if maxVisible <= 0 {
		maxVisible = popupMaxRows
	}

	paint := func(seq, text string) string {
		if seq == "" {
			return text
		}
		return seq + text + reset
	}

	var b strings.Builder
	b.WriteString(paint(mutedSeq, title))
	b.WriteString("\n")

	start := popupScrollStart(cursor, total, maxVisible)
	end := start + maxVisible
	if end > total {
		end = total
	}

	if start > 0 {
		b.WriteString(paint(mutedSeq, "  …"))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
	}

	rowCount := 0
	for i := start; i < end; i++ {
		prefix := "  "
		seq := mutedSeq
		if i == cursor {
			prefix = "> "
			seq = accentSeq
			if seq == "" {
				seq = mutedSeq
			}
		}
		b.WriteString(paint(seq, prefix+labels[i]))
		b.WriteString("\n")
		rowCount++
	}
	for rowCount < maxVisible {
		b.WriteString("\n")
		rowCount++
	}

	if end < total {
		b.WriteString(paint(mutedSeq, "  …"))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
	}

	return b.String()
}
