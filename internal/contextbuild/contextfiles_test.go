package contextbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stelmakhdigital/stell-ai"
)

func TestLoadContextSlots(t *testing.T) {
	root := t.TempDir()
	ctxDir := filepath.Join(root, ".stell", "context")
	if err := os.MkdirAll(ctxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "rules.md"), []byte("project rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	slots := LoadContextSlots("", root)
	if len(slots) != 1 || slots[0] != "project rules" {
		t.Fatalf("slots=%v", slots)
	}
}

func TestBuildSystemIncludesContextSlots(t *testing.T) {
	root := t.TempDir()
	ctxDir := filepath.Join(root, ".stell", "context")
	_ = os.MkdirAll(ctxDir, 0o755)
	_ = os.WriteFile(filepath.Join(ctxDir, "extra.md"), []byte("extra context"), 0o644)
	out := BuildSystem("", root, Options{})
	if !strings.Contains(out, "extra context") {
		t.Fatalf("missing context slot in system prompt: %q", out)
	}
}

func TestBuildSystemAppendsEnvironmentFooter(t *testing.T) {
	root := t.TempDir()
	out := BuildSystem("", root, Options{})
	footer := "Current working directory: " + filepath.ToSlash(root)
	if !strings.Contains(out, footer) {
		t.Fatalf("missing cwd footer in system prompt: %q", out)
	}
	if !strings.Contains(out, "Current date: ") {
		t.Fatalf("missing date footer in system prompt: %q", out)
	}
	lines := strings.Split(out, "\n")
	if got := lines[len(lines)-1]; got != footer {
		t.Fatalf("cwd must be the last line, got %q", got)
	}
}

func TestBuildSystemFooterSurvivesCustomSystemMD(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "SYSTEM.md"), []byte("custom prompt"), 0o644)
	out := BuildSystem("", root, Options{})
	if !strings.Contains(out, "custom prompt") {
		t.Fatalf("custom SYSTEM.md not used: %q", out)
	}
	if strings.Contains(out, "Available tools:") {
		t.Fatalf("tool sections must not be added to custom prompt: %q", out)
	}
	if !strings.Contains(out, "Current working directory: "+filepath.ToSlash(root)) {
		t.Fatalf("footer must be appended even with custom SYSTEM.md: %q", out)
	}
}

func TestDefaultSystemPromptToolsAndGuidelines(t *testing.T) {
	root := t.TempDir()
	out := BuildSystem("", root, Options{Tools: []ai.ToolDef{
		{Name: "read", PromptSnippet: "Read file contents", PromptGuidelines: []string{"Use read to examine files instead of cat or sed."}},
		{Name: "bash", PromptSnippet: "Execute bash commands (ls, grep, find, etc.)"},
		{Name: "hidden"}, // no snippet: must not appear in Available tools
	}})
	if !strings.Contains(out, "- read: Read file contents") {
		t.Fatalf("missing read snippet: %q", out)
	}
	if !strings.Contains(out, "- Use read to examine files instead of cat or sed.") {
		t.Fatalf("missing read guideline: %q", out)
	}
	if strings.Contains(out, "- hidden") {
		t.Fatalf("tool without snippet listed: %q", out)
	}
	// bash present without grep/find/ls -> conditional guideline
	if !strings.Contains(out, "- Use bash for file operations like ls, rg, find") {
		t.Fatalf("missing conditional bash guideline: %q", out)
	}
	if !strings.Contains(out, "- Be concise in your responses") {
		t.Fatalf("missing always-on guideline: %q", out)
	}
}

func TestDefaultSystemPromptNoBashGuidelineWithLs(t *testing.T) {
	root := t.TempDir()
	out := BuildSystem("", root, Options{Tools: []ai.ToolDef{
		{Name: "bash", PromptSnippet: "Execute bash commands"},
		{Name: "ls", PromptSnippet: "List directory contents"},
	}})
	if strings.Contains(out, "Use bash for file operations") {
		t.Fatalf("conditional bash guideline must be absent when ls is available: %q", out)
	}
}