package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/config"
)

func TestLoadProjectResources(t *testing.T) {
	root := t.TempDir()
	stell := filepath.Join(root, ".stell")
	skills := filepath.Join(stell, "skills", "hello-world")
	prompts := filepath.Join(stell, "prompts")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(prompts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skills, "SKILL.md"), []byte("---\nname: hello-world\ndescription: demo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prompts, "rus.md"), []byte("---\ndescription: rus\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Skills.List()) != 1 {
		t.Fatalf("skills = %v", cat.Skills.List())
	}
	if len(cat.Prompts.List()) != 1 {
		t.Fatalf("prompts = %v", cat.Prompts.List())
	}
}
