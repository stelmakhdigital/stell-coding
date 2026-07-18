package providerhooks

import (
	"context"
	"net/http"

	"github.com/stelmakhdigital/ai"
	"stell/agent/hooks"
)

// BusHTTPHooks адаптирует in-process шину хуков к ai.HTTPHooks. Provider-
// хуки (before_provider_headers, before_provider_request,
// after_provider_response) эмитятся в in-process шину и могут
// пробрасываться в подписанные subprocess-расширения (serializable payload).
func BusHTTPHooks(bus *hooks.Bus) ai.HTTPHooks {
	return &busHTTPHooks{bus: bus}
}

type busHTTPHooks struct {
	bus *hooks.Bus
}

func (b *busHTTPHooks) BeforeHeaders(ctx context.Context, provider, model string, h http.Header) {
	if !b.bus.HasSubscriber(hooks.BeforeProviderHeaders) {
		return
	}
	_ = b.bus.Emit(ctx, &hooks.Event{
		Name:    hooks.BeforeProviderHeaders,
		Payload: map[string]any{"provider": provider, "model": model},
		Header:  h,
	})
}

func (b *busHTTPHooks) BeforeRequest(ctx context.Context, provider, model string, body []byte) {
	if !b.bus.HasSubscriber(hooks.BeforeProviderRequest) {
		return
	}
	_ = b.bus.Emit(ctx, &hooks.Event{
		Name:    hooks.BeforeProviderRequest,
		Payload: map[string]any{"provider": provider, "model": model, "body": string(body)},
	})
}

func (b *busHTTPHooks) AfterResponse(ctx context.Context, provider, model string, status int, h http.Header) {
	if !b.bus.HasSubscriber(hooks.AfterProviderResponse) {
		return
	}
	_ = b.bus.Emit(ctx, &hooks.Event{
		Name:    hooks.AfterProviderResponse,
		Payload: map[string]any{"provider": provider, "model": model, "status": status},
		Header:  h,
	})
}
