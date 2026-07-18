package extensions

import "sync"

type UIRequest struct {
	ID   string
	Kind string
	Data map[string]any
}

type UIProtocol struct {
	mu      sync.Mutex
	pending map[string]chan map[string]any
	emit    func(UIRequest)
}

func NewUIProtocol(emit func(UIRequest)) *UIProtocol {
	return &UIProtocol{pending: map[string]chan map[string]any{}, emit: emit}
}

func (u *UIProtocol) Request(id, kind string, data map[string]any) map[string]any {
	if u == nil || u.emit == nil {
		return nil
	}
	ch := make(chan map[string]any, 1)
	u.mu.Lock()
	u.pending[id] = ch
	u.mu.Unlock()
	u.emit(UIRequest{ID: id, Kind: kind, Data: data})
	return <-ch
}

func (u *UIProtocol) Respond(id string, result map[string]any) {
	if u == nil {
		return
	}
	u.mu.Lock()
	ch := u.pending[id]
	delete(u.pending, id)
	u.mu.Unlock()
	if ch != nil {
		ch <- result
	}
}
