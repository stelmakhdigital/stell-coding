package tui

import "strings"

// simpleViewport — простой прокручиваемый viewport (дифференциальный), замена bubbles/viewport.
type simpleViewport struct {
	Width, Height int
	YOffset       int
	content       string
	lines         []string
}

func newViewport(w, h int) simpleViewport {
	return simpleViewport{Width: w, Height: h}
}

func (v *simpleViewport) SetContent(s string) {
	v.content = s
	v.lines = strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

func (v *simpleViewport) GotoBottom() {
	if v.Height <= 0 {
		v.YOffset = 0
		return
	}
	max := len(v.lines) - v.Height
	if max < 0 {
		max = 0
	}
	v.YOffset = max
}

func (v *simpleViewport) SetYOffset(y int) {
	if y < 0 {
		y = 0
	}
	max := len(v.lines) - v.Height
	if max < 0 {
		max = 0
	}
	if y > max {
		y = max
	}
	v.YOffset = y
}

func (v *simpleViewport) LineUp(n int) {
	v.SetYOffset(v.YOffset - n)
}

func (v *simpleViewport) LineDown(n int) {
	v.SetYOffset(v.YOffset + n)
}

func (v *simpleViewport) View() string {
	if v.Height <= 0 {
		return v.content
	}
	start := v.YOffset
	end := start + v.Height
	if start > len(v.lines) {
		start = len(v.lines)
	}
	if end > len(v.lines) {
		end = len(v.lines)
	}
	if start >= end {
		return ""
	}
	return strings.Join(v.lines[start:end], "\n")
}

func (v *simpleViewport) Update(msg Msg) (simpleViewport, Cmd) {
	switch m := msg.(type) {
	case KeyMsg:
		switch m.String() {
		case "up", "pgup":
			v.LineUp(1)
		case "down", "pgdown":
			v.LineDown(1)
		}
	}
	return *v, nil
}

func viewLineCount(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if s[len(s)-1] != '\n' {
		n++
	}
	return n
}
