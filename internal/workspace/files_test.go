package workspace

import "testing"

func TestFuzzyFilter(t *testing.T) {
	paths := []string{"internal/tui/tui.go", "README.md", "cmd/stell/main.go"}
	got := FuzzyFilter(paths, "tui", 10)
	if len(got) == 0 || got[0] != "internal/tui/tui.go" {
		t.Fatalf("unexpected fuzzy result: %v", got)
	}
}
