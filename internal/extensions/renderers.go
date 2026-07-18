package extensions

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// RendererRegistry хранит зарегистрированные расширениями форматтеры entry/message для TUI.
type RendererRegistry struct {
	mu               sync.RWMutex
	entryRenderers   map[string]RendererDef
	messageRenderers map[string]RendererDef
}

// RendererDef описывает, как рисовать кастомный entry/message.
type RendererDef struct {
	Label string
	Lines []string // optional static template lines; {content} placeholder
}

func NewRendererRegistry() *RendererRegistry {
	return &RendererRegistry{
		entryRenderers:   map[string]RendererDef{},
		messageRenderers: map[string]RendererDef{},
	}
}

func (r *RendererRegistry) RegisterEntryRenderer(customType, label string) {
	r.RegisterEntryRendererDef(customType, RendererDef{Label: label})
}

func (r *RendererRegistry) RegisterEntryRendererDef(customType string, def RendererDef) {
	if customType == "" {
		return
	}
	r.mu.Lock()
	r.entryRenderers[customType] = def
	r.mu.Unlock()
}

func (r *RendererRegistry) RegisterMessageRenderer(customType, label string) {
	r.RegisterMessageRendererDef(customType, RendererDef{Label: label})
}

func (r *RendererRegistry) RegisterMessageRendererDef(customType string, def RendererDef) {
	if customType == "" {
		return
	}
	r.mu.Lock()
	r.messageRenderers[customType] = def
	r.mu.Unlock()
}

func (r *RendererRegistry) EntryDef(customType string) (RendererDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.entryRenderers[customType]
	return d, ok
}

func (r *RendererRegistry) MessageDef(customType string) (RendererDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.messageRenderers[customType]
	return d, ok
}

// PaintEntry возвращает строки отображения для кастомного entry (тело expandable-карточки).
func (r *RendererRegistry) PaintEntry(customType, content string, data json.RawMessage, expanded bool) []string {
	r.mu.RLock()
	def := r.entryRenderers[customType]
	r.mu.RUnlock()
	label := def.Label
	if label == "" {
		if customType != "" {
			label = customType
		} else {
			label = "entry"
		}
	}
	header := fmt.Sprintf("[%s]", label)
	extra := parseDataSummary(data)
	if !expanded {
		summary := content
		if extra != "" {
			summary = content + " — " + extra
		}
		if summary == "" {
			return []string{header}
		}
		return []string{header + " " + trimOne(summary, 80)}
	}
	lines := []string{header}
	if len(def.Lines) > 0 {
		for _, ln := range def.Lines {
			lines = append(lines, strings.ReplaceAll(ln, "{content}", content))
		}
	} else if content != "" {
		lines = append(lines, strings.Split(content, "\n")...)
	}
	if extra != "" {
		lines = append(lines, "data: "+extra)
	}
	return lines
}

// PaintMessage возвращает строки отображения для кастомной message-карточки.
func (r *RendererRegistry) PaintMessage(customType, content string, expanded bool) []string {
	r.mu.RLock()
	def := r.messageRenderers[customType]
	r.mu.RUnlock()
	label := def.Label
	if label == "" {
		if customType == "" {
			return strings.Split(content, "\n")
		}
		label = customType
	}
	header := fmt.Sprintf("[%s]", label)
	if !expanded {
		if content == "" {
			return []string{header}
		}
		return []string{header + " " + trimOne(content, 80)}
	}
	lines := []string{header}
	if len(def.Lines) > 0 {
		for _, ln := range def.Lines {
			lines = append(lines, strings.ReplaceAll(ln, "{content}", content))
		}
	} else if content != "" {
		lines = append(lines, strings.Split(content, "\n")...)
	}
	return lines
}

func (r *RendererRegistry) FormatEntry(customType, content string, data json.RawMessage) string {
	r.mu.RLock()
	def := r.entryRenderers[customType]
	r.mu.RUnlock()
	label := def.Label
	if label == "" {
		if customType != "" {
			label = customType
		} else {
			label = "entry"
		}
	}
	extra := parseDataSummary(data)
	if len(def.Lines) == 0 {
		if extra != "" {
			return fmt.Sprintf("[%s] %s — %s", label, content, extra)
		}
		return fmt.Sprintf("[%s] %s", label, content)
	}
	return strings.Join(r.PaintEntry(customType, content, data, true), "\n")
}

func (r *RendererRegistry) FormatMessage(customType, content string) string {
	r.mu.RLock()
	def := r.messageRenderers[customType]
	r.mu.RUnlock()
	label := def.Label
	if label == "" {
		if customType != "" {
			label = customType
		} else {
			return content
		}
	}
	if len(def.Lines) == 0 {
		return fmt.Sprintf("[%s] %s", label, content)
	}
	return strings.Join(r.PaintMessage(customType, content, true), "\n")
}

func trimOne(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if n > 0 && len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

func parseDataSummary(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	var parts []string
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		if len(parts) >= 3 {
			break
		}
	}
	return strings.Join(parts, ", ")
}

// ShortcutDef — действие keybinding, зарегистрированное расширением.
type ShortcutDef struct {
	Key    string
	Action string
	Source string
}

// FlagDef — CLI/TUI flag, зарегистрированный расширением.
type FlagDef struct {
	Name        string
	Description string
	Type        string
	Default     any
	Source      string
}
