package tui

import (
	"strings"

	tuilib "stell/tui"
)

// Минимальные типы Msg/Cmd для интерактивного цикла (дифференциальный TUI-движок).

type Msg any

type Cmd func() Msg

func Batch(cmds ...Cmd) Cmd {
	var out []Cmd
	for _, c := range cmds {
		if c != nil {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return nil
	}
	if len(out) == 1 {
		return out[0]
	}
	return func() Msg {
		return batchMsg{cmds: out}
	}
}

type batchMsg struct{ cmds []Cmd }

type quitMsg struct{}

func Quit() Msg { return quitMsg{} }

// QuitCmd возвращает команду, завершающую interactive loop.
func QuitCmd() Cmd { return func() Msg { return quitMsg{} } }

type WindowSizeMsg struct {
	Width  int
	Height int
}

type KeyType int

const (
	KeyRunes KeyType = iota
	KeyEnter
	KeyEsc
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyTab
	KeyBackspace
	KeyCtrlC
	KeyPgUp
	KeyPgDown
	KeySpace
	KeyOther
)

type KeyMsg struct {
	Type  KeyType
	Runes []rune
	Alt   bool
	Paste bool
	raw   string
}

func (k KeyMsg) String() string {
	if k.raw != "" {
		return k.raw
	}
	switch k.Type {
	case KeyEnter:
		return "enter"
	case KeyEsc:
		return "esc"
	case KeyUp:
		return "up"
	case KeyDown:
		return "down"
	case KeyLeft:
		return "left"
	case KeyRight:
		return "right"
	case KeyTab:
		return "tab"
	case KeyBackspace:
		return "backspace"
	case KeyCtrlC:
		return "ctrl+c"
	case KeyPgUp:
		return "pgup"
	case KeyPgDown:
		return "pgdown"
	case KeySpace:
		return " "
	case KeyRunes:
		return string(k.Runes)
	default:
		return k.raw
	}
}

func parseKey(data string) KeyMsg {
	if id := tuilib.NormalizeKeyChord(tuilib.ParseKey(data)); id != "" {
		return keyMsgFromID(id)
	}
	if ch := tuilib.DecodePrintableKey(data); ch != "" {
		return KeyMsg{Type: KeyRunes, Runes: []rune(ch), raw: ch}
	}
	if strings.HasPrefix(data, "\x1b[200~") {
		return KeyMsg{Type: KeyRunes, Runes: []rune(data), raw: data, Paste: true}
	}
	if len(data) > 0 && data[0] == 0x1b {
		return KeyMsg{Type: KeyOther, raw: data}
	}
	return KeyMsg{Type: KeyRunes, Runes: []rune(data), raw: data, Paste: len(data) > 20}
}

func keyMsgFromID(id string) KeyMsg {
	switch id {
	case "enter":
		return KeyMsg{Type: KeyEnter, raw: "enter"}
	case "esc":
		return KeyMsg{Type: KeyEsc, raw: "esc"}
	case "up":
		return KeyMsg{Type: KeyUp, raw: "up"}
	case "down":
		return KeyMsg{Type: KeyDown, raw: "down"}
	case "left":
		return KeyMsg{Type: KeyLeft, raw: "left"}
	case "right":
		return KeyMsg{Type: KeyRight, raw: "right"}
	case "tab":
		return KeyMsg{Type: KeyTab, raw: "tab"}
	case "backspace":
		return KeyMsg{Type: KeyBackspace, raw: "backspace"}
	case "ctrl+c":
		return KeyMsg{Type: KeyCtrlC, raw: "ctrl+c"}
	case "pgup":
		return KeyMsg{Type: KeyPgUp, raw: "pgup"}
	case "pgdown":
		return KeyMsg{Type: KeyPgDown, raw: "pgdown"}
	case "space", " ":
		return KeyMsg{Type: KeySpace, raw: " "}
	default:
		return KeyMsg{Type: KeyOther, raw: id}
	}
}

type tickMsg struct{}
