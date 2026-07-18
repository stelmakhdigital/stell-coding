package extensions

import (
	"context"
	"fmt"
	"sync/atomic"
)

// WorkingIndicatorOpts соответствует опциям расширения working-indicator.
type WorkingIndicatorOpts struct {
	Label      string   `json:"label"`
	Message    string   `json:"message"`
	Frames     []string `json:"frames"`
	IntervalMs int      `json:"intervalMs"`
	Visible    *bool    `json:"visible"`
}

// UIHost связывает UI RPC subprocess расширения с host UIProtocol.
type UIHost struct {
	UI     *UIProtocol
	Themes func() []map[string]string
}

func (h *UIHost) ListThemes() []map[string]string {
	if h == nil {
		return nil
	}
	if h.Themes != nil {
		return h.Themes()
	}
	return nil
}

func (h *UIHost) Select(ctx context.Context, message string, options []string) (string, bool, error) {
	if h == nil || h.UI == nil {
		return "", false, fmt.Errorf("ui not configured")
	}
	id := fmt.Sprintf("ui-%d", uiID.Add(1))
	data := map[string]any{"message": message, "options": toAny(options)}
	res := h.UI.Request(id, "select", data)
	if res == nil {
		return "", true, nil
	}
	if cancelled, _ := res["cancelled"].(bool); cancelled {
		return "", true, nil
	}
	v, _ := res["value"].(string)
	return v, false, nil
}

func (h *UIHost) Confirm(ctx context.Context, message string) (bool, bool, error) {
	if h == nil || h.UI == nil {
		return false, false, fmt.Errorf("ui not configured")
	}
	id := fmt.Sprintf("ui-%d", uiID.Add(1))
	res := h.UI.Request(id, "confirm", map[string]any{"message": message})
	if res == nil {
		return false, true, nil
	}
	if cancelled, _ := res["cancelled"].(bool); cancelled {
		return false, true, nil
	}
	ok, _ := res["confirmed"].(bool)
	return ok, false, nil
}

func (h *UIHost) Input(ctx context.Context, message, placeholder, value string) (string, bool, error) {
	if h == nil || h.UI == nil {
		return "", false, fmt.Errorf("ui not configured")
	}
	id := fmt.Sprintf("ui-%d", uiID.Add(1))
	data := map[string]any{"message": message, "placeholder": placeholder}
	if value != "" {
		data["value"] = value
	}
	res := h.UI.Request(id, "input", data)
	if res == nil {
		return "", true, nil
	}
	if cancelled, _ := res["cancelled"].(bool); cancelled {
		return "", true, nil
	}
	v, _ := res["value"].(string)
	return v, false, nil
}

func (h *UIHost) SetHeader(text string) {
	if h == nil || h.UI == nil {
		return
	}
	id := fmt.Sprintf("ui-%d", uiID.Add(1))
	h.UI.Request(id, "setHeader", map[string]any{"content": text, "lines": splitLines(text)})
}

func (h *UIHost) SetWorkingIndicator(opts WorkingIndicatorOpts) {
	if h == nil || h.UI == nil {
		return
	}
	id := fmt.Sprintf("ui-%d", uiID.Add(1))
	label := opts.Label
	if label == "" {
		label = opts.Message
	}
	data := map[string]any{"label": label}
	if len(opts.Frames) > 0 {
		data["frames"] = opts.Frames
	}
	if opts.IntervalMs > 0 {
		data["intervalMs"] = opts.IntervalMs
	}
	if opts.Visible != nil {
		data["visible"] = *opts.Visible
	}
	h.UI.Request(id, "setWorkingIndicator", data)
}

func (h *UIHost) SetWorkingMessage(message string) {
	if h == nil || h.UI == nil {
		return
	}
	id := fmt.Sprintf("ui-%d", uiID.Add(1))
	h.UI.Request(id, "setWorkingMessage", map[string]any{"message": message, "label": message})
}

func (h *UIHost) SetWorkingVisible(visible *bool) {
	if h == nil || h.UI == nil {
		return
	}
	id := fmt.Sprintf("ui-%d", uiID.Add(1))
	data := map[string]any{}
	if visible != nil {
		data["visible"] = *visible
	}
	h.UI.Request(id, "setWorkingVisible", data)
}

var uiID atomic.Int64

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
