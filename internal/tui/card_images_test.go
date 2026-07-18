package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/agent"
)

func TestCardFromMessageAssistantImages(t *testing.T) {
	msg := ai.Message{
		Role:    ai.RoleAssistant,
		Content: "see image",
		Images:  []ai.ImageContent{{Type: "image", Data: "abc", MimeType: "image/png"}},
	}
	c, ok := cardFromMessage(msg, "", nil)
	if !ok || len(c.images) != 1 {
		t.Fatalf("got ok=%v images=%d", ok, len(c.images))
	}
}

func TestCardFromMessageToolImages(t *testing.T) {
	msg := ai.Message{
		Role:     ai.RoleTool,
		Content:  "result",
		ToolName: "read",
		Images:   []ai.ImageContent{{Type: "image", Data: "abc", MimeType: "image/png"}},
	}
	c, ok := cardFromMessage(msg, "", nil)
	if !ok || c.kind != cardTool || len(c.images) != 1 {
		t.Fatalf("got %+v ok=%v", c, ok)
	}
}

func TestOverlayKeyTreeFilter(t *testing.T) {
	m := &Model{overlayMode: overlayTree, overlayKeys: defaultOverlayKeyMap()}
	action, ok := m.overlayKeyAction("ctrl+u")
	if !ok || action != actionTreeFilterUserOnly {
		t.Fatalf("action=%q ok=%v", action, ok)
	}
	if action, ok := m.overlayKeyAction("ctrl+d"); !ok || action != actionTreeFilterDefault {
		t.Fatalf("tree ctrl+d action=%q ok=%v", action, ok)
	}
	m.overlayMode = overlaySession
	if action, ok := m.overlayKeyAction("ctrl+d"); !ok || action != actionSessionDelete {
		t.Fatalf("session ctrl+d action=%q ok=%v", action, ok)
	}
}

func TestSyncRetryStatusLine(t *testing.T) {
	m := &Model{
		keys:       DefaultKeybindings(),
		retryInfo:  &agent.AutoRetryInfo{Attempt: 1, MaxAttempts: 3},
		retryUntil: time.Now().Add(1500 * time.Millisecond),
	}
	m.syncRetryStatusLine()
	if m.statusLine == "" || !strings.Contains(m.statusLine, "Retry 1/3") {
		t.Fatalf("status=%q", m.statusLine)
	}
	if !strings.Contains(m.statusLine, "cancel") {
		t.Fatalf("missing cancel hint: %q", m.statusLine)
	}
}
