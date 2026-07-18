package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stell/coding-agent/internal/catalog"
	"stell/coding-agent/internal/config"
	"stell/coding-agent/internal/discovery"
	"stell/coding-agent/internal/prompts"
	"stell/coding-agent/internal/skills"
)

func TestPrepareMessageSkillExpansion(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: demo-skill
description: demo
---
Do the thing.`), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := skills.NewRegistry()
	reg.Scan([]catalog.Dir{{Path: root, Source: catalog.SourceProject}})

	svc := &Service{
		Config:  &config.Config{Settings: config.DefaultSettings()},
		Catalog: &discovery.Catalog{Skills: reg},
	}
	got := svc.PrepareMessage("/skill:demo-skill run it")
	if !strings.Contains(got, `<skill name="demo-skill"`) {
		t.Fatalf("expected skill block, got %q", got)
	}
	if !strings.Contains(got, "run it") {
		t.Fatalf("expected args appended, got %q", got)
	}
}

func TestPrepareMessagePromptExpansion(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.md"), []byte(`---
description: greet
---
Hello $1`), 0o644); err != nil {
		t.Fatal(err)
	}

	pr := prompts.NewRegistry()
	pr.Scan([]catalog.Dir{{Path: root, Source: catalog.SourceProject}})

	svc := &Service{
		Config:  &config.Config{Settings: config.DefaultSettings()},
		Catalog: &discovery.Catalog{Prompts: pr},
	}
	got := svc.PrepareMessage("/hello world")
	if got != "Hello world" {
		t.Fatalf("got %q", got)
	}
}
