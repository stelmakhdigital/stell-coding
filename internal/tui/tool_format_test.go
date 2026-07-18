package tui

import (
	"strings"
	"testing"
)

func TestFormatToolCallBash(t *testing.T) {
	got := formatToolCall("bash", map[string]any{"command": `python3 -c 'print("stell")'`})
	want := `bash: python3 -c 'print("stell")'`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatToolCallGeneric(t *testing.T) {
	got := formatToolCall("Read", map[string]any{"path": "main.go"})
	if got != "Read: main.go" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatToolResultTrustHint(t *testing.T) {
	got := formatToolResult("bash", "", "bash requires workspace trust or --approve", "")
	if !strings.Contains(got, "trust the workspace") {
		t.Fatalf("got %q", got)
	}
}
