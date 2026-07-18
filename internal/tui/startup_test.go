package tui

import (
	"strings"
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/agent"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-coding/internal/version"
)

func TestStartupLogoLine(t *testing.T) {
	version.Version = "0.1.0"
	m := Model{
		colors: defaultPalette(),
		svc:    &agent.Service{},
	}
	line := stripANSI(m.startupLogoLine())
	if !strings.Contains(line, "stell v0.1.0") {
		t.Fatalf("logo: %q", line)
	}
}

func TestStartupBannerQuiet(t *testing.T) {
	quiet := true
	m := Model{
		colors: defaultPalette(),
		svc:    &agent.Service{},
		cfg: &config.Config{
			Settings: config.Settings{QuietStartup: &quiet},
		},
	}
	if m.startupBanner() != "" {
		t.Fatal("expected empty quiet startup")
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
