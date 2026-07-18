package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
)

func TestSteerQueuePreservesImages(t *testing.T) {
	svc := &Service{
		streaming:    true,
		steeringMode: "one-at-a-time",
	}
	svc.steerQueue = append(svc.steerQueue, QueuedMessage{
		Text:   "look",
		Images: []ai.ImageContent{{Type: "image", Data: "abc", MimeType: "image/png"}},
	})
	text, imgs := svc.takeSteer()
	if text != "look" || len(imgs) != 1 {
		t.Fatalf("unexpected steer dequeue: text=%q imgs=%d", text, len(imgs))
	}
}

func TestSteerExpandAttachments(t *testing.T) {
	dir := t.TempDir()
	note := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(note, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := &Service{
		streaming:          true,
		steeringMode:       "one-at-a-time",
		pendingAttachments: []string{"note.txt"},
		Config:             &config.Config{Workspace: dir},
	}
	events := make(chan Event, 1)
	if err := svc.Prompt(t.Context(), "context", "steer", events); err != nil {
		t.Fatal(err)
	}
	<-events
	text, _ := svc.takeSteer()
	if !strings.Contains(text, "hello") {
		t.Fatalf("expected expanded file in steer message, got %q", text)
	}
}
