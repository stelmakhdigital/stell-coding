package extensions

import (
	"context"
	"encoding/json"
	"sync"
)

// AutocompleteProvider регистрируется subprocess расширения.
type AutocompleteProvider struct {
	ID     string
	Prefix string
	Label  string
	Owner  string
	Client *ProcessClient
}

type autocompleteRegistry struct {
	mu        sync.RWMutex
	providers []AutocompleteProvider
}

func newAutocompleteRegistry() *autocompleteRegistry {
	return &autocompleteRegistry{}
}

func (r *autocompleteRegistry) Register(p AutocompleteProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, existing := range r.providers {
		if existing.ID == p.ID {
			r.providers[i] = p
			return
		}
	}
	r.providers = append(r.providers, p)
}

func (r *autocompleteRegistry) UnregisterOwner(owner string) {
	if owner == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	filtered := r.providers[:0]
	for _, p := range r.providers {
		if p.Owner != owner {
			filtered = append(filtered, p)
		}
	}
	r.providers = filtered
}

func (r *autocompleteRegistry) Query(ctx context.Context, query string) []map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []map[string]string
	for _, p := range r.providers {
		if p.Client == nil {
			continue
		}
		if p.Prefix != "" && !hasPrefix(query, p.Prefix) {
			continue
		}
		raw, err := p.Client.Call(ctx, "autocomplete/query", map[string]any{
			"query":  query,
			"prefix": p.Prefix,
		})
		if err != nil {
			continue
		}
		var res struct {
			Suggestions []struct {
				Label string `json:"label"`
				Value string `json:"value"`
				Desc  string `json:"description"`
			} `json:"suggestions"`
		}
		if json.Unmarshal(raw, &res) != nil {
			continue
		}
		for _, s := range res.Suggestions {
			out = append(out, map[string]string{
				"label": s.Label,
				"value": s.Value,
				"desc":  s.Desc,
			})
		}
	}
	return out
}

func hasPrefix(q, prefix string) bool {
	if prefix == "" {
		return true
	}
	return len(q) >= len(prefix) && q[:len(prefix)] == prefix
}
