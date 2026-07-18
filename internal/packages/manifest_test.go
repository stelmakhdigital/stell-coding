package packages

import (
	"strings"
	"testing"
)

func TestParseSource(t *testing.T) {
	t.Run("rejects npm", func(t *testing.T) {
		_, err := ParseSource("npm:demo-stell@0.1.0")
		if err == nil {
			t.Fatal("expected error for npm source")
		}
		if !strings.Contains(err.Error(), "not supported") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("git with ref", func(t *testing.T) {
		s, err := ParseSource("git:github.com/org/pkg@v1.0.0")
		if err != nil {
			t.Fatal(err)
		}
		if s.Kind != "git" || s.Ref != "v1.0.0" {
			t.Fatalf("source = %+v", s)
		}
		if s.Path != "https://github.com/org/pkg" {
			t.Fatalf("path = %q", s.Path)
		}
	})

	t.Run("local path", func(t *testing.T) {
		s, err := ParseSource("./packages/demo-stell")
		if err != nil {
			t.Fatal(err)
		}
		if s.Kind != "local" || s.Path != "./packages/demo-stell" {
			t.Fatalf("source = %+v", s)
		}
	})
}
