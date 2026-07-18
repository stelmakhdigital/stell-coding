package extensions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestJSON)
	if err := os.WriteFile(path, []byte(`{
  "name": "echo",
  "type": "process-jsonrpc",
  "command": ["go", "run", "."],
  "hooks": ["before_agent_start"]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "echo" || len(m.Command) != 3 {
		t.Fatalf("manifest = %+v", m)
	}
	if !m.Subscribes("before_agent_start") {
		t.Fatal("expected before_agent_start subscription")
	}
}

func TestLoadManifestYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ManifestYAML), []byte(`name: echo
type: process-jsonrpc
command: ["go", "run", "."]
hooks: [before_agent_start]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "echo" {
		t.Fatalf("name = %q", m.Name)
	}
}
