package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/catalog"
)

func TestScanAndPromptXML(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: demo-skill
description: A test skill
---
Body`), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry()
	reg.Scan([]catalog.Dir{{Path: root, Source: catalog.SourceProject}})
	if len(reg.List()) != 1 {
		t.Fatalf("list = %v", reg.List())
	}
	xml := reg.PromptXML()
	if !strings.Contains(xml, "<available_skills>") {
		t.Fatalf("xml = %q", xml)
	}
	if !strings.Contains(xml, "demo-skill") {
		t.Fatalf("xml = %q", xml)
	}
}

func TestDisableModelInvocation(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "hidden-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: hidden-skill
description: hidden
disable-model-invocation: true
---
Body`), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry()
	reg.Scan([]catalog.Dir{{Path: root, Source: catalog.SourceProject}})
	if reg.PromptXML() != "" {
		t.Fatalf("hidden skill should not appear in prompt xml: %q", reg.PromptXML())
	}
}

func TestExpandCommand(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(`---
name: demo-skill
description: demo
---
Instructions here.`), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry()
	reg.Scan([]catalog.Dir{{Path: root, Source: catalog.SourceProject}})
	got := ExpandCommand(reg, "/skill:demo-skill args")
	if !strings.Contains(got, "Instructions here.") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "args") {
		t.Fatalf("got %q", got)
	}
}
