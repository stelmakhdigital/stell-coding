package sdk_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stelmakhdigital/stell-ai"
	coreagent "github.com/stelmakhdigital/stell-agent"
	"github.com/stelmakhdigital/stell-coding/sdk"
)

func TestCreateSessionOptsStreamFnOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"text_delta\",\"delta\":\"sdk\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"done\",\"reason\":\"stop\"}\n\n")
	}))
	defer srv.Close()

	fn := coreagent.StreamFn(func(ctx context.Context, req ai.ChatRequest) (<-chan ai.ChatEvent, error) {
		ch := make(chan ai.ChatEvent, 2)
		go func() {
			defer close(ch)
			ch <- ai.ChatEvent{Type: ai.EventToken, Token: "sdk"}
			ch <- ai.ChatEvent{Type: ai.EventDone, StopReason: "completed"}
		}()
		return ch, nil
	})

	opts := sdk.Options{
		Workspace: t.TempDir(),
		NoTools:   "all",
		StreamFn:  fn,
	}
	sess, err := sdk.CreateSessionOpts(opts)
	if err != nil {
		t.Skip(err) // окружение без models
	}
	if sess.Service.StreamFn == nil {
		t.Fatal("StreamFn not set on Service")
	}
	ch, err := sess.Service.StreamFn(context.Background(), ai.ChatRequest{Model: "x"})
	if err != nil {
		t.Fatal(err)
	}
	var tok string
	for ev := range ch {
		if ev.Type == ai.EventToken {
			tok += ev.Token
		}
	}
	if tok != "sdk" {
		t.Fatalf("tok=%q", tok)
	}
	_ = srv
}
