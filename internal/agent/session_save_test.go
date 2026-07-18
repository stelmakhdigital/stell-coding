package agent

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stelmakhdigital/stell-agent/session"
)

func TestSaveSessionReportsError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "notadir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	prev := sessionSaveErrOut
	sessionSaveErrOut = &buf
	t.Cleanup(func() { sessionSaveErrOut = prev })

	a := &Agent{
		Sessions: session.NewManager(dir),
		SessPath: filepath.Join(blocker, "session.jsonl"),
	}
	a.saveSession()

	out := buf.String()
	if !strings.Contains(out, "failed to save session") {
		t.Fatalf("stderr = %q, want save failure notice", out)
	}
	if !strings.Contains(out, a.SessPath) {
		t.Fatalf("stderr = %q, want path %q", out, a.SessPath)
	}
}

func TestSaveSessionIfNeededReportsError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "notadir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	prev := sessionSaveErrOut
	sessionSaveErrOut = &buf
	t.Cleanup(func() { sessionSaveErrOut = prev })

	s := &Service{
		Sessions: session.NewManager(dir),
		SessPath: filepath.Join(blocker, "session.jsonl"),
	}
	s.saveSessionIfNeeded()

	if !strings.Contains(buf.String(), "failed to save session") {
		t.Fatalf("stderr = %q, want save failure notice", buf.String())
	}
}
