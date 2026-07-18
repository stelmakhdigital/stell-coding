package main

import (
	"strings"
	"testing"
)

func TestFormatResumeCommandSessionOnly(t *testing.T) {
	got := formatResumeCommand("./stell", "/tmp/sessions/chat.jsonl", "", "/tmp/project")
	want := "Resume: ./stell --session /tmp/sessions/chat.jsonl"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatResumeCommandWithExplicitWorkspace(t *testing.T) {
	got := formatResumeCommand("stell", "/tmp/sessions/chat.jsonl", "/other/ws", "/tmp/project")
	if !strings.Contains(got, "--workspace") {
		t.Fatalf("expected --workspace in %q", got)
	}
	if !strings.Contains(got, "/other/ws") {
		t.Fatalf("expected explicit workspace in %q", got)
	}
}

func TestFormatResumeCommandMatchingWorkspaceOmitted(t *testing.T) {
	got := formatResumeCommand("stell", "/tmp/sessions/chat.jsonl", "/tmp/project", "/tmp/project")
	want := "Resume: stell --session /tmp/sessions/chat.jsonl"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatResumeCommandQuotesSpaces(t *testing.T) {
	got := formatResumeCommand("./stell", "/tmp/my sessions/chat.jsonl", "", "/tmp/project")
	if !strings.Contains(got, `"`) {
		t.Fatalf("expected quoted path in %q", got)
	}
}

func TestFormatResumeCommandEmptyPath(t *testing.T) {
	if got := formatResumeCommand("stell", "", "", ""); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}
