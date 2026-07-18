package extensions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type GrantRequest struct {
	ID    string
	Key   string
	Perms ExtPermissions
}

type GrantBroker struct {
	mu       sync.Mutex
	pending  map[string]chan bool
	timeout  time.Duration
	onEmit   func(GrantRequest)
	onStderr func(GrantRequest) (bool, error)
}

func NewGrantBroker() *GrantBroker {
	return &GrantBroker{
		pending: map[string]chan bool{},
		timeout: 5 * time.Minute,
	}
}

func (b *GrantBroker) SetEmitter(fn func(GrantRequest)) { b.onEmit = fn }

func (b *GrantBroker) SetStderrFallback(fn func(GrantRequest) (bool, error)) {
	b.onStderr = fn
}

func (b *GrantBroker) Request(ctx context.Context, key string, perms ExtPermissions) (bool, error) {
	req := GrantRequest{ID: "ext-grant-" + randHex(6), Key: key, Perms: perms}
	ch := make(chan bool, 1)
	b.mu.Lock()
	b.pending[req.ID] = ch
	emit := b.onEmit
	fallback := b.onStderr
	b.mu.Unlock()

	if emit != nil {
		emit(req)
	} else if fallback != nil {
		return fallback(req)
	}
	select {
	case ok := <-ch:
		return ok, nil
	case <-time.After(b.timeout):
		b.cleanup(req.ID)
		return false, nil
	case <-ctx.Done():
		b.cleanup(req.ID)
		return false, ctx.Err()
	}
}

func (b *GrantBroker) Respond(id string, granted bool) error {
	b.mu.Lock()
	ch, ok := b.pending[id]
	if ok {
		delete(b.pending, id)
	}
	b.mu.Unlock()
	if !ok {
		return nil
	}
	ch <- granted
	return nil
}

func (b *GrantBroker) cleanup(id string) {
	b.mu.Lock()
	delete(b.pending, id)
	b.mu.Unlock()
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type ExtPermissions struct {
	Shell   bool     `json:"shell,omitempty"`
	Network bool     `json:"network,omitempty"`
	Paths   []string `json:"paths,omitempty"`
}
