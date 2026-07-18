package tui

import "testing"

func TestOverlayStackPushPop(t *testing.T) {
	m := &Model{}
	m.pushOverlayFrame(overlayFrame{mode: overlayTree, text: "tree"})
	if m.overlayMode != overlayTree || m.overlay != "tree" {
		t.Fatalf("top=%v %q", m.overlayMode, m.overlay)
	}
	m.pushOverlayFrame(overlayFrame{mode: overlaySettings, text: "settings"})
	if m.overlayMode != overlaySettings || len(m.overlayStack) != 1 {
		t.Fatalf("nested mode=%v stack=%d", m.overlayMode, len(m.overlayStack))
	}
	m.closeOverlay()
	if m.overlayMode != overlayTree || m.overlay != "tree" {
		t.Fatalf("restore failed: mode=%v overlay=%q", m.overlayMode, m.overlay)
	}
	m.closeOverlay()
	if m.overlayMode != overlayNone || m.overlay != "" {
		t.Fatalf("clear failed: mode=%v overlay=%q", m.overlayMode, m.overlay)
	}
}
