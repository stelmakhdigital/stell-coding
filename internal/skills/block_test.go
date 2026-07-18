package skills

import (
	"strings"
	"testing"
)

func TestParseFormatSkillBlockRoundtrip(t *testing.T) {
	orig := FormatSkillBlock("hello-world", "/path/to/SKILL.md", "/path/to", "Do the thing.", "run it")
	got, ok := ParseSkillBlock(orig)
	if !ok {
		t.Fatal("parse failed")
	}
	if got.Name != "hello-world" || got.Location != "/path/to/SKILL.md" {
		t.Fatalf("meta = %+v", got)
	}
	if !strings.Contains(got.Content, "Do the thing.") {
		t.Fatalf("content = %q", got.Content)
	}
	if got.UserMessage != "run it" {
		t.Fatalf("user = %q", got.UserMessage)
	}
}
