package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentsSkillsDirsWalksUp(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, "repo")
	nested := filepath.Join(gitDir, "pkg", "sub")
	agents := filepath.Join(gitDir, ".agents", "skills")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	dirs := AgentsSkillsDirs(nested)
	if len(dirs) != 1 || dirs[0].Path != agents {
		t.Fatalf("dirs = %v want %q", dirs, agents)
	}
}
