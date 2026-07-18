package packages

import (
	"testing"
)

func TestCheckRecordUpdateSkipsPinnedAndLocal(t *testing.T) {
	m := NewManager(t.TempDir(), t.TempDir(), "project")
	local, ok := m.checkRecordUpdate(t.Context(), Record{
		Name:   "local-pkg",
		Source: "/tmp/foo",
	})
	if ok || local.Name != "" {
		t.Fatalf("local: %+v %v", local, ok)
	}
	pinned, ok := m.checkRecordUpdate(t.Context(), Record{
		Name:   "pinned",
		Source: "git:github.com/org/pkg@v1.0.0",
	})
	if ok || pinned.Name != "" {
		t.Fatalf("pinned: %+v %v", pinned, ok)
	}
}

func TestGitDisplayName(t *testing.T) {
	if got := gitDisplayName("https://github.com/org/pkg.git"); got != "github.com/org/pkg" {
		t.Fatalf("got %q", got)
	}
}
