package tui

import (
	"fmt"
	"strconv"
	"strings"
)

type palette struct {
	Accent     string
	Foreground string
	Muted      string
	UserBlock  string
	Assistant  string
	Tool       string
	Error      string
	Border     string
	// Токены темы (раскрытые).
	Tokens map[string]string
}

func defaultPalette() palette {
	return palette{
		Accent:     "86",
		Foreground: "252",
		Muted:      "245",
		UserBlock:  "236",
		Assistant:  "159",
		Tool:       "117",
		Error:      "203",
		Border:     "238",
		Tokens:     map[string]string{},
	}
}

func colorSeq(c string) string {
	if c == "" {
		return ""
	}
	if strings.HasPrefix(c, "#") && len(c) == 7 {
		r, _ := strconv.ParseInt(c[1:3], 16, 0)
		g, _ := strconv.ParseInt(c[3:5], 16, 0)
		b, _ := strconv.ParseInt(c[5:7], 16, 0)
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
	}
	if n, err := strconv.Atoi(c); err == nil {
		return fmt.Sprintf("\x1b[38;5;%dm", n)
	}
	return ""
}

func bgSeq(c string) string {
	if c == "" {
		return ""
	}
	if strings.HasPrefix(c, "#") && len(c) == 7 {
		r, _ := strconv.ParseInt(c[1:3], 16, 0)
		g, _ := strconv.ParseInt(c[3:5], 16, 0)
		b, _ := strconv.ParseInt(c[5:7], 16, 0)
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
	}
	if n, err := strconv.Atoi(c); err == nil {
		return fmt.Sprintf("\x1b[48;5;%dm", n)
	}
	return ""
}

const reset = "\x1b[0m"

type ansiStyle struct {
	fg, bg string
	bold   bool
	italic bool
	pad    int
}

func (s ansiStyle) Render(text string) string {
	var b strings.Builder
	if s.bold {
		b.WriteString("\x1b[1m")
	}
	if s.italic {
		b.WriteString("\x1b[3m")
	}
	b.WriteString(bgSeq(s.bg))
	b.WriteString(colorSeq(s.fg))
	if s.pad > 0 {
		b.WriteString(strings.Repeat(" ", s.pad))
	}
	b.WriteString(text)
	if s.pad > 0 {
		b.WriteString(strings.Repeat(" ", s.pad))
	}
	b.WriteString(reset)
	return b.String()
}

func (p palette) header() ansiStyle {
	return ansiStyle{fg: p.Accent, bold: true}
}

func (p palette) accent() ansiStyle {
	return ansiStyle{fg: p.Accent, bold: true}
}

func (p palette) muted() ansiStyle {
	return ansiStyle{fg: p.Muted}
}

func (p palette) thinking() ansiStyle {
	return ansiStyle{fg: p.tokenColor("thinkingText", p.Muted), italic: true}
}

// thinkingStyleSeq — italic + thinkingText fg (thinking-трассы).
func (p palette) thinkingStyleSeq() string {
	return "\x1b[3m" + colorSeq(p.tokenColor("thinkingText", p.Muted))
}

// styleThinkingLines оборачивает каждую строку в thinking italic+muted и reinject после SGR reset.
func styleThinkingLines(s string, p palette) string {
	if s == "" {
		return ""
	}
	seq := p.thinkingStyleSeq()
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		line = reinjectBgAfterReset(line, seq)
		lines[i] = seq + line + reset
	}
	return strings.Join(lines, "\n")
}

func (p palette) assistant() ansiStyle {
	return ansiStyle{fg: p.Assistant}
}

func (p palette) tool() ansiStyle {
	return ansiStyle{fg: p.Tool}
}

func (p palette) err() ansiStyle {
	return ansiStyle{fg: p.Error}
}

func (p palette) tokenColor(key, fallback string) string {
	if p.Tokens != nil {
		if v := p.Tokens[key]; v != "" {
			return v
		}
	}
	return fallback
}

func (p palette) toolStatusBg(st cardStatus) string {
	switch st {
	case cardStatusPending:
		return p.tokenColor("toolPendingBg", "#1e1e2e")
	case cardStatusError:
		return p.tokenColor("toolErrorBg", "#2e1e1e")
	case cardStatusSuccess:
		return p.tokenColor("toolSuccessBg", "#1e2e1e")
	default:
		return p.tokenColor("toolSuccessBg", "#1e2e1e")
	}
}

func (p palette) toolOutput() ansiStyle {
	fg := p.tokenColor("toolOutput", "")
	if fg == "" {
		fg = p.Muted
	}
	return ansiStyle{fg: fg}
}

func (p palette) diffAdded() ansiStyle {
	return ansiStyle{fg: p.tokenColor("toolDiffAdded", p.Tool)}
}

func (p palette) diffRemoved() ansiStyle {
	return ansiStyle{fg: p.tokenColor("toolDiffRemoved", p.Error)}
}

func (p palette) diffContext() ansiStyle {
	return ansiStyle{fg: p.tokenColor("toolDiffContext", p.Muted)}
}

// paintLinesWithBg применяет цвет фона к каждой строке как full-width блок.
// Дополняет строки пробелами до width, чтобы bg был сплошным прямоугольником, а не облегал текст.
func paintLinesWithBg(s, bgColor string, width int) string {
	seq := bgSeq(bgColor)
	if seq == "" {
		return s
	}
	if width <= 0 {
		width = 1
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		pad := width - visibleLen(line)
		if pad < 0 {
			pad = 0
		}
		padded := line + strings.Repeat(" ", pad)
		padded = reinjectBgAfterReset(padded, seq)
		lines[i] = seq + padded + reset
	}
	return strings.Join(lines, "\n")
}

// reinjectBgAfterReset снова применяет bg CSI после полного SGR reset, чтобы padding/spaces сохраняли цвет блока.
func reinjectBgAfterReset(s, bgSeq string) string {
	if bgSeq == "" || !strings.Contains(s, "\x1b[0m") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + len(bgSeq)*2)
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+3 <= len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) {
				c := s[j]
				j++
				if c >= 0x40 && c <= 0x7e {
					break
				}
			}
			csi := s[i:j]
			b.WriteString(csi)
			// Полный reset сбрасывает фон; восстанавливаем block bg до конца строки.
			if csi == "\x1b[0m" && j < len(s) {
				b.WriteString(bgSeq)
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func (p palette) chip() ansiStyle {
	return ansiStyle{fg: p.Foreground, bg: p.UserBlock, pad: 1}
}

func (p palette) chipSelected() ansiStyle {
	return ansiStyle{fg: p.Foreground, bg: p.UserBlock, pad: 1, bold: true}
}

func styleWidth(s string) int {
	n := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) {
					c := s[j]
					j++
					if c >= 0x40 && c <= 0x7e {
						break
					}
				}
				i = j
				continue
			}
		}
		n++
		i++
	}
	return n
}
