package tui

import (
	"strings"

	"github.com/mattn/go-runewidth"
	tuilib "stell/tui"
)

func wrapText(text string, width int) string {
	if width <= 0 || text == "" {
		return text
	}
	parts := strings.Split(text, "\n")
	for i, part := range parts {
		parts[i] = wrapParagraph(part, width)
	}
	return strings.Join(parts, "\n")
}

func wrapParagraph(text string, width int) string {
	if width <= 0 {
		return text
	}
	if runewidth.StringWidth(text) <= width {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var cur strings.Builder
	curW := 0

	flush := func() {
		if cur.Len() == 0 {
			return
		}
		lines = append(lines, cur.String())
		cur.Reset()
		curW = 0
	}

	for _, word := range words {
		wordW := runewidth.StringWidth(word)
		if wordW > width {
			flush()
			lines = append(lines, hardWrapWord(word, width)...)
			continue
		}
		extra := wordW
		if cur.Len() > 0 {
			extra++
		}
		if curW+extra > width {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
			curW++
		}
		cur.WriteString(word)
		curW += wordW
	}
	flush()
	return strings.Join(lines, "\n")
}

func hardWrapWord(word string, width int) []string {
	if width <= 0 {
		return []string{word}
	}
	var lines []string
	var cur strings.Builder
	curW := 0
	for _, r := range word {
		rw := runewidth.RuneWidth(r)
		if rw > width {
			if cur.Len() > 0 {
				lines = append(lines, cur.String())
				cur.Reset()
				curW = 0
			}
			lines = append(lines, string(r))
			continue
		}
		if curW+rw > width {
			lines = append(lines, cur.String())
			cur.Reset()
			curW = 0
		}
		cur.WriteRune(r)
		curW += rw
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}


func truncate(s string, width int) string {
	return tuilib.Truncate(s, width)
}

func visibleLen(s string) int {
	return tuilib.VisibleLen(s)
}
