package tui

import (
	"fmt"
	"strings"

	"stell/coding-agent/internal/extensions"
)

type uiOverlayState struct {
	id     string
	method string
	data   map[string]any
	input  string
}

func (m *Model) openUIOverlay(req extensions.UIRequest) {
	switch req.Kind {
	case "notify":
		m.handleFireForgetUI(req)
		return
	case "setStatus", "setWidget", "setTitle", "setHeader", "set_editor_text", "setWorkingIndicator", "setWorkingMessage", "setWorkingVisible", "workingIndicator", "replaceEditor", "setEditor":
		m.handleFireForgetUI(req)
		return
	case "customPaint":
		if m.uiOverlay != nil && m.uiOverlay.method == "custom" {
			sid, _ := req.Data["sessionId"].(string)
			if sid == "" || sid == m.uiOverlay.id {
				if lines := stringSlice(req.Data["lines"]); len(lines) > 0 {
					m.uiOverlay.data["lines"] = anyStrings(lines)
					m.uiOverlay.data["content"] = strings.Join(lines, "\n")
					m.overlay = m.renderUIOverlay()
				}
			}
		}
		return
	}
	initial := ""
	if v, ok := req.Data["value"].(string); ok {
		initial = v
	}
	mode, _ := req.Data["mode"].(string)
	m.uiOverlay = &uiOverlayState{
		id:     req.ID,
		method: req.Kind,
		data:   req.Data,
		input:  initial,
	}
	if req.Kind == "custom" && (mode == "replaceEditor" || mode == "replace") {
		lines := stringSlice(req.Data["lines"])
		if content, _ := req.Data["content"].(string); content != "" && len(lines) == 0 {
			lines = strings.Split(content, "\n")
		}
		title, _ := req.Data["title"].(string)
		body := strings.Join(lines, "\n")
		if title != "" {
			body = title + "\n" + body
		}
		m.extReplaceEditor = body
		// Оставляем uiOverlay для обработки клавиш без текста modal overlay.
		return
	}
	m.pushOverlayFrame(overlayFrame{
		mode:      overlayUI,
		text:      m.renderUIOverlay(),
		uiOverlay: m.uiOverlay,
	})
}

func anyStrings(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func (m *Model) handleFireForgetUI(req extensions.UIRequest) {
	msg, _ := req.Data["message"].(string)
	if msg == "" {
		msg, _ = req.Data["text"].(string)
	}
	switch req.Kind {
	case "setTitle":
		if title, _ := req.Data["title"].(string); title != "" {
			m.extTitle = title
		} else if msg != "" {
			m.extTitle = msg
		}
	case "setHeader":
		if content, _ := req.Data["content"].(string); content != "" {
			m.extHeader = content
		} else if lines := stringSlice(req.Data["lines"]); len(lines) > 0 {
			m.extHeader = strings.Join(lines, "\n")
		} else if msg != "" {
			m.extHeader = msg
		}
	case "setWidget", "setStatus":
		placement, _ := req.Data["placement"].(string)
		if placement == "" {
			placement, _ = req.Data["position"].(string)
		}
		text := msg
		if widget, _ := req.Data["widget"].(string); widget != "" {
			text = widget
		}
		if status, _ := req.Data["status"].(string); status != "" && text == "" {
			text = status
		}
		switch strings.ToLower(placement) {
		case "above", "aboveEditor", "above_editor":
			m.extAbove = text
		case "below", "belowEditor", "below_editor":
			m.extBelow = text
		case "footer":
			m.extFooter = text
		case "working", "workingIndicator":
			m.extWorking = text
		default:
			m.extWidget = text
		}
	case "setWorkingIndicator", "workingIndicator", "setWorkingMessage":
		if label, _ := req.Data["label"].(string); label != "" {
			m.extWorking = label
		} else if msg != "" {
			m.extWorking = msg
		}
		if frames := stringSlice(req.Data["frames"]); len(frames) > 0 {
			m.extWorkingFrames = frames
		}
		if ms, ok := req.Data["intervalMs"].(float64); ok && ms > 0 {
			m.extWorkingEveryMs = int(ms)
		}
	case "setWorkingVisible":
		if visible, ok := req.Data["visible"].(bool); ok && !visible {
			m.extWorking = ""
			m.extWorkingFrames = nil
		}
	case "set_editor_text":
		if text, _ := req.Data["text"].(string); text != "" {
			m.composer.SetValue(text)
		} else if msg != "" {
			m.composer.SetValue(msg)
		}
	case "replaceEditor", "setEditor":
		if content, _ := req.Data["content"].(string); content != "" {
			m.extReplaceEditor = content
		} else if lines := stringSlice(req.Data["lines"]); len(lines) > 0 {
			m.extReplaceEditor = strings.Join(lines, "\n")
		} else if msg != "" {
			m.extReplaceEditor = msg
		}
		if clear, _ := req.Data["clear"].(bool); clear {
			m.extReplaceEditor = ""
		}
	default:
		if msg != "" {
			m.addInfo("ext: " + msg)
		}
	}
	if m.svc.Extensions != nil {
		m.svc.Extensions.RespondUI(req.ID, map[string]any{})
	}
}

func (m *Model) renderUIOverlay() string {
	if m.uiOverlay == nil {
		return ""
	}
	u := m.uiOverlay
	var b strings.Builder
	switch u.method {
	case "select":
		b.WriteString("select (↑/↓, enter, esc cancel)\n")
		if msg, _ := u.data["message"].(string); msg != "" {
			b.WriteString(msg)
			b.WriteString("\n")
		}
		opts := stringSlice(u.data["options"])
		for i, o := range opts {
			prefix := "  "
			if i == m.overlayCursor {
				prefix = "> "
			}
			b.WriteString(prefix)
			b.WriteString(o)
			b.WriteString("\n")
		}
	case "confirm":
		msg, _ := u.data["message"].(string)
		b.WriteString("confirm (y/n, esc cancel)\n")
		b.WriteString(msg)
		b.WriteString("\n")
	case "input", "editor":
		msg, _ := u.data["message"].(string)
		b.WriteString(u.method)
		b.WriteString(" (enter submit, esc cancel)\n")
		b.WriteString(msg)
		b.WriteString("\n> ")
		b.WriteString(u.input)
		b.WriteString("_")
	case "custom":
		b.WriteString("extension panel (keys → extension, esc close)\n")
		if title, _ := u.data["title"].(string); title != "" {
			b.WriteString(title)
			b.WriteString("\n")
		}
		if content, _ := u.data["content"].(string); content != "" {
			b.WriteString(content)
		} else if lines := stringSlice(u.data["lines"]); len(lines) > 0 {
			b.WriteString(strings.Join(lines, "\n"))
		}
	case "customPaint":
		// обрабатывается как fire-and-forget update
		return ""
	case "notify":
		msg, _ := u.data["message"].(string)
		b.WriteString("notify: ")
		b.WriteString(msg)
		b.WriteString("\n(press any key)")
	default:
		fmt.Fprintf(&b, "extension UI: %s\n(esc close)", u.method)
	}
	return b.String()
}

func (m *Model) handleUIOverlayKey(key string) bool {
	if m.uiOverlay == nil {
		return false
	}
	u := m.uiOverlay
	switch u.method {
	case "select":
		opts := stringSlice(u.data["options"])
		switch key {
		case "esc":
			m.respondUI(map[string]any{"cancelled": true})
			return true
		case "up", "shift+tab":
			if m.overlayCursor > 0 {
				m.overlayCursor--
			}
			m.overlay = m.renderUIOverlay()
			return true
		case "down", "tab":
			if m.overlayCursor < len(opts)-1 {
				m.overlayCursor++
			}
			m.overlay = m.renderUIOverlay()
			return true
		case "enter":
			if len(opts) > 0 && m.overlayCursor < len(opts) {
				m.respondUI(map[string]any{"value": opts[m.overlayCursor]})
			}
			return true
		}
	case "confirm":
		switch key {
		case "y", "Y":
			m.respondUI(map[string]any{"confirmed": true})
			return true
		case "n", "N", "esc":
			if key == "esc" {
				m.respondUI(map[string]any{"cancelled": true})
			} else {
				m.respondUI(map[string]any{"confirmed": false})
			}
			return true
		}
	case "input", "editor":
		switch key {
		case "esc":
			m.respondUI(map[string]any{"cancelled": true})
			return true
		case "enter":
			m.respondUI(map[string]any{"value": u.input})
			return true
		case "backspace":
			if len(u.input) > 0 {
				u.input = u.input[:len(u.input)-1]
				m.overlay = m.renderUIOverlay()
			}
			return true
		default:
			if len(key) == 1 {
				u.input += key
				m.overlay = m.renderUIOverlay()
				return true
			}
		}
	case "notify", "setStatus", "setWidget", "setTitle", "set_editor_text",
		"setWorkingIndicator", "workingIndicator", "replaceEditor", "setEditor":
		m.respondUI(nil)
		return true
	case "custom":
		if key == "esc" {
			if m.svc.Extensions != nil && m.svc.Extensions.CustomSessions != nil {
				m.svc.Extensions.CustomSessions.Done(u.id, map[string]any{"cancelled": true})
			}
			m.extReplaceEditor = ""
			m.respondUI(map[string]any{"cancelled": true})
			return true
		}
		if m.svc.Extensions != nil {
			m.svc.Extensions.NotifyCustomKey(u.id, key)
		}
		return true
	}
	return false
}

func (m *Model) respondUI(result map[string]any) {
	if m.uiOverlay == nil || m.svc.Extensions == nil {
		m.closeOverlay()
		return
	}
	id := m.uiOverlay.id
	if result != nil {
		m.svc.Extensions.RespondUI(id, result)
	}
	m.uiOverlay = nil
	m.closeOverlay()
}

func stringSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
