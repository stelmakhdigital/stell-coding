package packages

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallLocalPackage(t *testing.T) {
	src := t.TempDir()
	writePackage(t, src, "demo-pkg", "0.1.0")

	global := t.TempDir()
	project := t.TempDir()
	mgr := NewManager(global, project, "project")

	rec, err := mgr.Install(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Name != "demo-pkg" || rec.Version != "0.1.0" {
		t.Fatalf("rec = %+v", rec)
	}
	dirs, err := mgr.EnabledResourceDirs("skills")
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 1 {
		t.Fatalf("skill dirs = %v", dirs)
	}
}

func writePackage(t *testing.T, root, name, version string) {
	t.Helper()
	skills := filepath.Join(root, "skills", "demo-skill")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skills, "SKILL.md"), []byte(`---
name: demo-skill
description: Demo skill for tests
---
Do demo things.`), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"name": name, "version": version,
		"keywords": []string{"stell-package"},
		"stell": map[string]any{"skills": []string{"./skills"}},
	}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "package.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}
