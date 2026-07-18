package extensions

import (
	"sync"
	"sync/atomic"
)

// CustomUISlot описывает регистрацию кастомного UI-компонента.
// Расширения регистрируют через host/ui/register_custom; interactive host
// рендерит их через дерево TUI Component.
type CustomUISlot struct {
	ID       string         `json:"id"`
	Owner    string         `json:"owner"`
	Label    string         `json:"label,omitempty"`
	Lines    []string       `json:"lines,omitempty"`
	Props    map[string]any `json:"props,omitempty"`
	ThemeKey string         `json:"themeKey,omitempty"`
}

// CustomUIRegistry хранит UI-слоты расширений по id.
type CustomUIRegistry struct {
	slots map[string]CustomUISlot
	mu    sync.RWMutex
}

func NewCustomUIRegistry() *CustomUIRegistry {
	return &CustomUIRegistry{slots: map[string]CustomUISlot{}}
}

func (r *CustomUIRegistry) Register(slot CustomUISlot) {
	if r == nil || slot.ID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.slots == nil {
		r.slots = map[string]CustomUISlot{}
	}
	r.slots[slot.ID] = slot
}

func (r *CustomUIRegistry) Unregister(id string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.slots, id)
	r.mu.Unlock()
}

func (r *CustomUIRegistry) Get(id string) (CustomUISlot, bool) {
	if r == nil {
		return CustomUISlot{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.slots[id]
	return s, ok
}

func (r *CustomUIRegistry) List() []CustomUISlot {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]CustomUISlot, 0, len(r.slots))
	for _, s := range r.slots {
		out = append(out, s)
	}
	return out
}

// CustomSession — живая кастомная UI-поверхность (overlay или replace-editor).
type CustomSession struct {
	ID       string
	Owner    string
	Mode     string // overlay | replaceEditor
	Title    string
	Lines    []string
	Props    map[string]any
	Done     bool
	Result   map[string]any
}

// CustomSessionManager ведёт interactive custom UI-сессии для paint/key RPC.
type CustomSessionManager struct {
	mu       sync.Mutex
	sessions map[string]*CustomSession
	seq      atomic.Uint64
	onPaint  func(id string, lines []string)
	onDone   func(id string, result map[string]any)
}

func NewCustomSessionManager() *CustomSessionManager {
	return &CustomSessionManager{sessions: map[string]*CustomSession{}}
}

func (m *CustomSessionManager) SetHooks(onPaint func(id string, lines []string), onDone func(id string, result map[string]any)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.onPaint = onPaint
	m.onDone = onDone
	m.mu.Unlock()
}

func (m *CustomSessionManager) Open(owner, mode, title string, lines []string, props map[string]any) *CustomSession {
	if m == nil {
		return nil
	}
	if mode == "" {
		mode = "overlay"
	}
	id := fmtID(m.seq.Add(1))
	s := &CustomSession{
		ID: id, Owner: owner, Mode: mode, Title: title,
		Lines: append([]string(nil), lines...), Props: props,
	}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	return s
}

func (m *CustomSessionManager) Get(id string) *CustomSession {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[id]
}

func (m *CustomSessionManager) Paint(id string, lines []string) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok || s.Done {
		m.mu.Unlock()
		return false
	}
	s.Lines = append([]string(nil), lines...)
	cb := m.onPaint
	m.mu.Unlock()
	if cb != nil {
		cb(id, lines)
	}
	return true
}

func (m *CustomSessionManager) Done(id string, result map[string]any) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return false
	}
	s.Done = true
	s.Result = result
	delete(m.sessions, id)
	cb := m.onDone
	m.mu.Unlock()
	if cb != nil {
		cb(id, result)
	}
	return true
}

func (m *CustomSessionManager) Close(id string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

func fmtID(n uint64) string {
	return "custom-" + itoa(n)
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
