package config

import (
	"runtime"
	"testing"
)

func TestMaxToolIterationsDefaultUnlimited(t *testing.T) {
	s := DefaultSettings()
	if got := s.MaxToolIterations(); got != 0 {
		t.Fatalf("default MaxToolIterations = %d, want 0 (unlimited)", got)
	}
}

func TestMaxToolIterationsExplicitValue(t *testing.T) {
	s := Settings{Agent: AgentSettings{MaxToolIterations: 42}}
	if got := s.MaxToolIterations(); got != 42 {
		t.Fatalf("MaxToolIterations = %d, want 42", got)
	}
}

func TestExternalEditorCommandPrecedence(t *testing.T) {
	t.Setenv("VISUAL", "visual-editor")
	t.Setenv("EDITOR", "editor")

	s := Settings{ExternalEditor: "code --wait"}
	if got := s.ExternalEditorCommand(); got != "code --wait" {
		t.Fatalf("ExternalEditorCommand = %q, want code --wait", got)
	}

	s = Settings{}
	if got := s.ExternalEditorCommand(); got != "visual-editor" {
		t.Fatalf("ExternalEditorCommand = %q, want visual-editor", got)
	}

	t.Setenv("VISUAL", "")
	if got := s.ExternalEditorCommand(); got != "editor" {
		t.Fatalf("ExternalEditorCommand = %q, want editor", got)
	}

	t.Setenv("EDITOR", "")
	want := "nano"
	if runtime.GOOS == "windows" {
		want = "notepad"
	}
	if got := s.ExternalEditorCommand(); got != want {
		t.Fatalf("ExternalEditorCommand = %q, want %s", got, want)
	}
}

func TestMarkdownPagerCommandPrecedence(t *testing.T) {
	t.Setenv("PAGER", "more")
	s := Settings{MarkdownPager: "less -R"}
	if got := s.MarkdownPagerCommand(); got != "less -R" {
		t.Fatalf("MarkdownPagerCommand = %q, want less -R", got)
	}
	s = Settings{}
	if got := s.MarkdownPagerCommand(); got != "more" {
		t.Fatalf("MarkdownPagerCommand = %q, want more", got)
	}
	t.Setenv("PAGER", "less")
	if got := s.MarkdownPagerCommand(); got != "less -R" {
		t.Fatalf("bare less should get -R, got %q", got)
	}
	t.Setenv("PAGER", "")
	if got := s.MarkdownPagerCommand(); got != "less -R" {
		t.Fatalf("default = %q, want less -R", got)
	}
}

func TestMergeSettingsExternalEditor(t *testing.T) {
	base := Settings{ExternalEditor: "vim"}
	merged := mergeSettings(base, Settings{ExternalEditor: "code --wait"})
	if merged.ExternalEditor != "code --wait" {
		t.Fatalf("merged ExternalEditor = %q, want code --wait", merged.ExternalEditor)
	}
}

func TestMergeSettingsMarkdownPager(t *testing.T) {
	base := Settings{MarkdownPager: "more"}
	merged := mergeSettings(base, Settings{MarkdownPager: "less -R"})
	if merged.MarkdownPager != "less -R" {
		t.Fatalf("merged MarkdownPager = %q, want less -R", merged.MarkdownPager)
	}
}

func TestMergeSettingsKeepsAgentCap(t *testing.T) {
	base := Settings{Agent: AgentSettings{MaxToolIterations: 10}}
	merged := mergeSettings(base, Settings{})
	if got := merged.MaxToolIterations(); got != 10 {
		t.Fatalf("merged MaxToolIterations = %d, want 10", got)
	}
	merged = mergeSettings(base, Settings{Agent: AgentSettings{MaxToolIterations: 20}})
	if got := merged.MaxToolIterations(); got != 20 {
		t.Fatalf("merged MaxToolIterations = %d, want 20", got)
	}
}
