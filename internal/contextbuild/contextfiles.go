package contextbuild

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadContextSlots читает опциональные markdown-сниппеты из .stell/context/ (проект) и ~/.stell/agent/context/ (global).
func LoadContextSlots(globalDir, workspace string) []string {
	var slots []string
	if globalDir != "" {
		slots = append(slots, readContextDir(filepath.Join(globalDir, "context"))...)
	}
	if workspace != "" {
		slots = append(slots, readContextDir(filepath.Join(workspace, ".stell", "context"))...)
	}
	return slots
}

func readContextDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if text := strings.TrimSpace(string(data)); text != "" {
			out = append(out, text)
		}
	}
	return out
}
