package config

import (
	"os"
	"testing"
)

func TestDiffScrollEnabledDefaultOn(t *testing.T) {
	s := DefaultSettings()
	if !s.DiffScrollEnabled() {
		t.Fatal("diffScroll should default on")
	}
	s.DiffScroll = ptrBool(false)
	if s.DiffScrollEnabled() {
		t.Fatal("diffScroll should respect false")
	}
}

func TestNormalizeTreeFilterMode(t *testing.T) {
	if got := NormalizeTreeFilterMode("no-tools"); got != "noTools" {
		t.Fatalf("got %q", got)
	}
}

func TestOutputPadDefault(t *testing.T) {
	s := DefaultSettings()
	if s.OutputPadOrDefault() != 1 {
		t.Fatalf("got %d", s.OutputPadOrDefault())
	}
}

func TestApplyHTTPProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	ApplyHTTPProxy("http://127.0.0.1:7890")
	if got := os.Getenv("HTTP_PROXY"); got != "http://127.0.0.1:7890" {
		t.Fatalf("HTTP_PROXY=%q", got)
	}
}

func ptrBool(v bool) *bool { return &v }
