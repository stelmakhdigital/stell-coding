package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSettingsProfileExternalEditor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{
  "schemaVersion": 1,
  "defaultProfile": "default",
  "profiles": {
    "default": {
      "externalEditor": "code --wait"
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, ok, err := readSettings(path)
	if err != nil || !ok {
		t.Fatalf("readSettings: ok=%v err=%v", ok, err)
	}
	if s.ExternalEditor != "code --wait" {
		t.Fatalf("ExternalEditor = %q, want code --wait", s.ExternalEditor)
	}
	if got := s.ExternalEditorCommand(); got != "code --wait" {
		t.Fatalf("ExternalEditorCommand = %q", got)
	}
}

func TestReadSettingsTopLevelOverridesProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{
  "externalEditor": "vim",
  "defaultProfile": "default",
  "profiles": {
    "default": {
      "externalEditor": "code --wait"
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, ok, err := readSettings(path)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if s.ExternalEditor != "vim" {
		t.Fatalf("ExternalEditor = %q, want vim", s.ExternalEditor)
	}
}
