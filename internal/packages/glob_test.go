package packages

import "testing"

func TestMatchInclude(t *testing.T) {
	patterns := []string{"*.md", "!secret.md"}
	if !MatchInclude(patterns, "readme.md") {
		t.Fatal("expected readme.md included")
	}
	if MatchInclude(patterns, "secret.md") {
		t.Fatal("expected secret.md excluded")
	}
}
