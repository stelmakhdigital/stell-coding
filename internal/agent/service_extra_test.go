package agent

import (
	"testing"

	"stell/coding-agent/internal/config"
)

func TestNeedsAutoCompactRespectsToggle(t *testing.T) {
	settings := config.DefaultSettings()
	enabled := false
	svc := &Service{Config: &config.Config{Settings: settings}, autoCompact: &enabled}
	if svc.AutoCompactionEnabled() {
		t.Fatal("expected auto compact disabled")
	}
}

func TestCycleThinkingLevel(t *testing.T) {
	svc := &Service{thinkingLevel: "off"}
	if lv := svc.CycleThinkingLevel(); lv != "low" {
		t.Fatalf("got %q", lv)
	}
}
