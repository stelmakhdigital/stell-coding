package extensions

import (
	"context"
	"testing"
)

func TestCancelWorkflow(t *testing.T) {
	s := NewSupervisor("/tmp", nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	s.RegisterWorkflow("run-1", cancel)

	if err := s.CancelWorkflow("run-1"); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() == nil {
		t.Fatal("workflow cancel did not propagate")
	}

	if err := s.CancelWorkflow("missing"); err == nil {
		t.Fatal("expected error for missing workflow")
	}
}

func TestWorkflowNotify(t *testing.T) {
	s := NewSupervisor("/tmp", nil, nil)
	ch := make(chan map[string]any, 1)
	s.SetWorkflowNotify(func(method string, params map[string]any) {
		params["_method"] = method
		ch <- params
	})
	s.handleExtensionNotify(nil, "workflow/register", map[string]any{
		"runId": "w1", "title": "test", "steps": []any{"a"},
	})
	select {
	case data := <-ch:
		if data["runId"] != "w1" {
			t.Fatalf("expected runId w1, got %v", data["runId"])
		}
	default:
		t.Fatal("expected workflow notify")
	}
}
