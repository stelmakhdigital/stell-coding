package tui

import (
	"strings"
	"testing"

	"stell/coding-agent/internal/update"
)

func TestShowNewVersionNotification(t *testing.T) {
	m := Model{colors: defaultPalette()}
	m.showNewVersionNotification(update.LatestRelease{Version: "1.2.3"})
	if len(m.startupNotices) != 1 {
		t.Fatalf("notices: %d", len(m.startupNotices))
	}
	if m.startupNotices[0].skillName != "Update Available" {
		t.Fatalf("title: %q", m.startupNotices[0].skillName)
	}
	if !strings.Contains(m.startupNotices[0].body, "1.2.3") {
		t.Fatalf("body: %q", m.startupNotices[0].body)
	}
}

func TestShowPackageUpdateNotification(t *testing.T) {
	m := Model{colors: defaultPalette()}
	m.showPackageUpdateNotification([]string{"pkg-a", "pkg-b"})
	if len(m.startupNotices) != 1 {
		t.Fatal("expected one notice")
	}
	if !strings.Contains(m.startupNotices[0].body, "pkg-a") {
		t.Fatalf("body: %q", m.startupNotices[0].body)
	}
}
