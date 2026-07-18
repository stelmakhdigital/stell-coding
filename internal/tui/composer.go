package tui

type composerState struct {
	ed     *Editor
	height int
	keys   *KeybindingsManager
}

func newComposer() composerState {
	ed := NewEditor()
	ed.SetPlaceholder("Message…  (/help)")
	ed.SetFocused(true)
	return composerState{ed: ed, height: 3, keys: NewKeybindingsManager(DefaultTUIKeybindings(), nil)}
}

func (c composerState) Value() string {
	if c.ed == nil {
		return ""
	}
	return c.ed.Value()
}

func (c *composerState) SetValue(v string) {
	if c.ed == nil {
		*c = newComposer()
	}
	c.ed.SetValue(v)
}

func (c *composerState) SetWidth(w int)  {}
func (c *composerState) SetHeight(h int) { c.height = h }
func (c *composerState) Height() int {
	if c.height <= 0 {
		return 3
	}
	return c.height
}
func (c *composerState) Blur() { c.ed.SetFocused(false) }
func (c *composerState) Focus() Cmd {
	c.ed.SetFocused(true)
	return nil
}
func (c *composerState) Focused() bool { return c.ed.Focused() }

func (c *composerState) View() string {
	lines := c.ed.Render(80)
	return joinLines(lines)
}

func (c *composerState) Update(msg Msg) (composerState, Cmd) {
	switch m := msg.(type) {
	case KeyMsg:
		switch m.Type {
		case KeyEnter:
			// submit обрабатывает Model
		case KeyBackspace:
			c.ed.HandleInput("\x7f")
		case KeyLeft:
			c.ed.HandleInput("\x1b[D")
		case KeyRight:
			c.ed.HandleInput("\x1b[C")
		case KeyUp:
			c.ed.HandleInput("\x1b[A")
		case KeyDown:
			c.ed.HandleInput("\x1b[B")
		case KeyRunes, KeySpace, KeyOther:
			if m.String() == "enter" || m.String() == "alt+enter" {
				break
			}
			// Пробрасываем CSI / именованные последовательности в редактор.
			if m.raw != "" {
				switch {
				case c.keys != nil && c.keys.Matches(m.raw, "tui.editor.jumpForward"):
					c.ed.HandleInput("\x1d")
				case c.keys != nil && c.keys.Matches(m.raw, "tui.editor.jumpBackward"):
					c.ed.HandleInput("\x1b\x1d")
				case m.raw == "ctrl+left":
					c.ed.HandleInput("\x1b[1;5D")
				case m.raw == "ctrl+right":
					c.ed.HandleInput("\x1b[1;5C")
				case m.raw == "ctrl+a":
					c.ed.HandleInput("\x01")
				case m.raw == "ctrl+e":
					c.ed.HandleInput("\x05")
				case m.raw == "alt+pgup", m.raw == "alt+pgdown", m.raw == "ctrl+y", m.raw == "ctrl+d", m.raw == "ctrl+z", m.raw == "shift+enter", m.raw == "pgup", m.raw == "pgdown":
					// Действия уровня app; оставляем для Model (или editor page через bindings).
				default:
					c.ed.HandleInput(m.raw)
				}
			} else if len(m.Runes) > 0 {
				c.ed.HandleInput(string(m.Runes))
			}
		}
	}
	return *c, nil
}

func (m *Model) resizeComposer() {}

func (m *Model) clearComposer() {
	m.composer.SetValue("")
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for i := 1; i < len(lines); i++ {
		out += "\n" + lines[i]
	}
	return out
}
