package rpc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"stell/coding-agent/internal/agent"
	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
	"github.com/stelmakhdigital/ai/provider"
	"github.com/stelmakhdigital/ai/provider/mock"
	"stell/agent/session"
	"stell/agent/tools"

	_ "github.com/stelmakhdigital/ai/provider/mock"
)

func TestPromptRPC(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock", Model: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}

	mp := mock.New("mock",
		[]ai.ChatEvent{mock.Token("Hello"), mock.Done(1, 1)},
	)
	reg := provider.NewRegistry()
	reg.Register(cfg.Models[0], mp)

	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	if err := tools.RegisterBuiltins(rt); err != nil {
		t.Fatal(err)
	}

	sess := session.NewManager(dir)
	svc := agent.NewService(cfg, reg, rt, sess, "", cfg.Models[0], nil, nil)

	var inBuf bytes.Buffer
	var outBuf syncBuffer
	inBuf.WriteString(`{"type":"prompt","id":"1","message":"hi"}`)
	inBuf.WriteString("\n")

	srv := &Server{
		In:     &inBuf,
		Out:    &outBuf,
		ErrOut: io.Discard,
		Svc:    svc,
		Cfg:    cfg,
		Models: cfg.Models,
	}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.respEnc = json.NewEncoder(srv.Out)
	srv.eventEnc.SetEscapeHTML(false)
	srv.respEnc.SetEscapeHTML(false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	waitForResponse(t, &outBuf, "prompt", true)

	lines := drainLines(&outBuf)
	var sawStart, sawDelta, sawSettled bool
	for _, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		switch obj["type"] {
		case "agent_start":
			sawStart = true
		case "message_update":
			if ev, _ := obj["assistantMessageEvent"].(map[string]any); ev != nil {
				if ev["type"] == "text_delta" && ev["delta"] == "Hello" {
					sawDelta = true
				}
			}
		case "agent_settled":
			sawSettled = true
		}
	}

	if !sawStart || !sawDelta || !sawSettled {
		t.Fatalf("events missing: start=%v delta=%v settled=%v\nlines=%v", sawStart, sawDelta, sawSettled, lines)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not finish")
	}
}

// syncBuffer защищает bytes.Buffer, чтобы тест мог читать вывод, пока
// горутина сервера ещё пишет.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

func waitForResponse(t *testing.T, out *syncBuffer, command string, success bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, line := range strings.Split(out.String(), "\n") {
			if line == "" {
				continue
			}
			var resp map[string]any
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				continue
			}
			if resp["type"] == "response" && resp["command"] == command && resp["success"] == success {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("response for %q not found in %q", command, out.String())
}

func drainLines(out *syncBuffer) []string {
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(out.Bytes()))
	for sc.Scan() {
		if s := sc.Text(); s != "" {
			lines = append(lines, s)
		}
	}
	return lines
}

type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

func TestEmitLogsEncodeError(t *testing.T) {
	var errBuf bytes.Buffer
	srv := &Server{
		Out:    errWriter{err: io.ErrClosedPipe},
		ErrOut: &errBuf,
	}
	srv.eventEnc = json.NewEncoder(srv.Out)
	srv.emit(map[string]any{"type": "agent_start"})
	if !strings.Contains(errBuf.String(), "encode event") {
		t.Fatalf("ErrOut = %q, want encode event notice", errBuf.String())
	}
}

func TestRespondLogsEncodeError(t *testing.T) {
	var errBuf bytes.Buffer
	srv := &Server{
		Out:    errWriter{err: io.ErrClosedPipe},
		ErrOut: &errBuf,
	}
	srv.respEnc = json.NewEncoder(srv.Out)
	srv.respond("1", "prompt", true, nil, "")
	if !strings.Contains(errBuf.String(), "encode response") {
		t.Fatalf("ErrOut = %q, want encode response notice", errBuf.String())
	}
}
