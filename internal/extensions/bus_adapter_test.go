package extensions

import (
	"context"
	"testing"

	"github.com/stelmakhdigital/stell-agent/hooks"
)

func TestApplyHookResponse(t *testing.T) {
	ev := &hooks.Event{Name: hooks.ToolCall}
	applyHookResponse(ev, map[string]any{
		"appendSystem": "extra",
		"cancel":       true,
		"block":        true,
		"args":         map[string]any{"path": "b.go"},
		"text":         "rewritten",
		"command":      "ls -la",
	})
	if ev.AppendSystem != "extra" || !ev.Cancel || !ev.Block {
		t.Fatalf("event = %+v", ev)
	}
	if ev.Args["path"] != "b.go" || ev.Text != "rewritten" || ev.Command != "ls -la" {
		t.Fatalf("event = %+v", ev)
	}

	// nil response leaves the event untouched
	ev2 := &hooks.Event{Name: hooks.Input, Text: "keep"}
	applyHookResponse(ev2, nil)
	if ev2.Text != "keep" || ev2.Cancel {
		t.Fatalf("event2 = %+v", ev2)
	}
}

func TestAttachBusAllowsProviderHooksWhenSubscribed(t *testing.T) {
	s := NewSupervisor(t.TempDir(), nil, nil)
	bus := hooks.NewBus()
	s.AttachBus(bus)

	// no running extensions → no dynamic interest at all
	if bus.HasSubscriber(hooks.ToolCall) {
		t.Fatal("no extensions running, must have no subscribers")
	}
	// Provider hooks are forwardable when an extension declares interest;
	// with zero extensions they remain unsubscribed.
	if bus.HasSubscriber(hooks.BeforeProviderRequest) {
		t.Fatal("no extensions: provider hooks should not have subscribers")
	}

	ev := &hooks.Event{Name: hooks.BeforeProviderHeaders}
	if err := bus.Emit(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
}
