package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"stell/coding-agent/internal/agent"
	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/themes"
)

func TestToolCardStatusBackground(t *testing.T) {
	m, _ := testModel(t)
	m.width = 80
	m.activeTheme = themes.DefaultTheme()
	m.colors = paletteFromTheme(m.activeTheme)

	pending := card{kind: cardTool, toolName: "bash", toolPath: "ls", status: cardStatusPending, startedAt: time.Now()}
	success := card{kind: cardTool, toolName: "bash", toolPath: "ls", toolContent: "ok", status: cardStatusSuccess, startedAt: time.Now().Add(-time.Second), endedAt: time.Now()}
	errCard := card{kind: cardTool, toolName: "bash", toolPath: "ls", toolContent: "fail", status: cardStatusError, startedAt: time.Now().Add(-time.Second), endedAt: time.Now()}

	p := m.renderCardForTest(0, pending, 80)
	s := m.renderCardForTest(0, success, 80)
	e := m.renderCardForTest(0, errCard, 80)
	if !strings.Contains(p, "\x1b[48;") {
		t.Fatalf("pending should have bg: %q", p)
	}
	if p == s || s == e || p == e {
		t.Fatal("pending/success/error renders should differ")
	}
	pendingBg := m.colors.toolStatusBg(cardStatusPending)
	successBg := m.colors.toolStatusBg(cardStatusSuccess)
	errorBg := m.colors.toolStatusBg(cardStatusError)
	if pendingBg == successBg || successBg == errorBg {
		t.Fatalf("bg tokens should differ: %q %q %q", pendingBg, successBg, errorBg)
	}
}

func TestPaintLinesWithBgFullWidthBlock(t *testing.T) {
	const w = 40
	bg := "#1e2e1e"
	in := "short\n" + strings.Repeat("x", 30) + "\n" + "\x1b[90mTook 0.2s\x1b[0m"
	out := paintLinesWithBg(in, bg, w)
	if !strings.Contains(out, "\x1b[48;") {
		t.Fatalf("missing bg CSI: %q", out)
	}
	for i, line := range strings.Split(out, "\n") {
		if got := visibleLen(line); got != w {
			t.Fatalf("line %d visibleLen=%d want %d: %q", i, got, w, line)
		}
	}
	// After muted reset, bg must be reinjected so trailing pad keeps block color.
	tookLine := strings.Split(out, "\n")[2]
	resetIdx := strings.Index(tookLine, "\x1b[0m")
	if resetIdx < 0 {
		t.Fatalf("expected reset in Took line: %q", tookLine)
	}
	afterReset := tookLine[resetIdx+len("\x1b[0m"):]
	if !strings.Contains(afterReset, "\x1b[48;") {
		t.Fatalf("bg should resume after Took reset for padding: %q", tookLine)
	}
}

func TestToolCardRenderFullWidthBg(t *testing.T) {
	m, _ := testModel(t)
	m.width = 80
	m.cellW = 8
	m.activeTheme = themes.DefaultTheme()
	m.colors = paletteFromTheme(m.activeTheme)
	c := card{
		kind: cardBash, toolName: "bash", toolPath: "ls",
		toolContent: "a\nlonger-output-line-here",
		status:      cardStatusSuccess,
		startedAt:   time.Now().Add(-time.Second),
		endedAt:     time.Now(),
	}
	const w = 60
	out := m.renderCardForTest(0, c, w)
	if !strings.Contains(out, "\x1b[48;") {
		t.Fatalf("missing bg: %q", out)
	}
	prev := m.width
	m.width = w
	lay := m.chatLayout()
	m.width = prev
	want := lay.contentW + lay.blockMargin
	for i, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if got := visibleLen(line); got != want {
			t.Fatalf("line %d visibleLen=%d want %d: %q", i, got, want, line)
		}
	}
}

func TestElapsedAndTookFooter(t *testing.T) {
	m, _ := testModel(t)
	m.width = 80
	m.activeTheme = themes.DefaultTheme()
	m.colors = paletteFromTheme(m.activeTheme)

	start := time.Now().Add(-2500 * time.Millisecond)
	pending := card{
		kind: cardBash, toolName: "bash", toolPath: "sleep", status: cardStatusPending, startedAt: start,
	}
	out := m.renderCardForTest(0, pending, 80)
	if !strings.Contains(out, "Elapsed") {
		t.Fatalf("pending should show Elapsed, got %q", out)
	}

	done := card{
		kind: cardBash, toolName: "bash", toolPath: "sleep", toolContent: "done",
		status: cardStatusSuccess, startedAt: start, endedAt: start.Add(6 * time.Second),
	}
	out2 := m.renderCardForTest(0, done, 80)
	if !strings.Contains(out2, "Took 6.0s") {
		t.Fatalf("done should show Took 6.0s, got %q", out2)
	}
	if strings.Contains(out2, "Elapsed") {
		t.Fatal("done should not show Elapsed")
	}
}

func TestReadShowsBody(t *testing.T) {
	m, _ := testModel(t)
	m.width = 80
	m.activeTheme = themes.DefaultTheme()
	m.colors = paletteFromTheme(m.activeTheme)

	c := card{
		kind: cardTool, toolName: "read", toolPath: "main.go",
		toolContent: "package main\n\nfunc main() {}\n",
		status:      cardStatusSuccess,
		startedAt:   time.Now(),
		endedAt:     time.Now(),
	}
	out := m.renderCardForTest(0, c, 80)
	plain := stripANSIForTest(out)
	if !strings.Contains(plain, "package main") {
		t.Fatalf("read should show body, got %q", plain)
	}
	if !strings.Contains(out, "read") {
		t.Fatalf("read should show header, got %q", out)
	}
	if strings.Contains(out, "to expand") {
		t.Fatalf("should not collapse: %q", out)
	}
}

func stripANSIForTest(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) {
					c := s[j]
					j++
					if c >= 0x40 && c <= 0x7e {
						break
					}
				}
				i = j
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func TestBashShowsFullOutput(t *testing.T) {
	m, _ := testModel(t)
	m.width = 40
	m.activeTheme = themes.DefaultTheme()
	m.colors = paletteFromTheme(m.activeTheme)

	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("line-%d-%s", i, strings.Repeat("x", 8)))
	}
	c := card{
		kind: cardBash, toolName: "bash", toolPath: "seq",
		toolContent: strings.Join(lines, "\n"),
		status:      cardStatusSuccess,
		startedAt:   time.Now(),
		endedAt:     time.Now(),
	}
	out := m.renderCardForTest(0, c, 40)
	if strings.Contains(out, "earlier") || strings.Contains(out, "to expand") {
		t.Fatalf("bash should not collapse: %q", out)
	}
	plain := stripANSIForTest(out)
	if !strings.Contains(plain, "line-0-") || !strings.Contains(plain, "line-19-") {
		t.Fatalf("full bash output should keep first and last lines: %q", plain)
	}
}

func TestStripRunningHeartbeat(t *testing.T) {
	in := "hello\n[running 5s / 60s]\nworld"
	got := stripRunningHeartbeat(in)
	if strings.Contains(got, "[running") {
		t.Fatalf("heartbeat not stripped: %q", got)
	}
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Fatalf("content lost: %q", got)
	}
}

func TestToolCallSetsPendingStatus(t *testing.T) {
	m := newTestEventModel()
	m.applyEvent(agent.Event{
		Type:     agent.EventToolCall,
		ToolCall: &ai.ToolCall{Name: "bash", Args: map[string]any{"command": "ls", "timeout": float64(60)}},
	})
	if len(m.lines) != 1 || m.lines[0].status != cardStatusPending {
		t.Fatalf("want pending tool card, got %+v", m.lines)
	}
	if m.lines[0].timeoutSec != 60 {
		t.Fatalf("timeoutSec=%d", m.lines[0].timeoutSec)
	}
	m.applyEvent(agent.Event{
		Type: agent.EventToolResult,
		ToolResult: &agent.ToolResult{
			Name:    "bash",
			Content: "ok",
		},
	})
	if m.lines[0].status != cardStatusSuccess {
		t.Fatalf("status=%v want success", m.lines[0].status)
	}
	if m.lines[0].endedAt.IsZero() {
		t.Fatal("endedAt should be set")
	}
}

// Smoke: EventToolCall → render includes status bg CSI and live Elapsed footer.
func TestEventToolCallRenderElapsedAndBg(t *testing.T) {
	m, _ := testModel(t)
	m.width = 80
	m.activeTheme = themes.DefaultTheme()
	m.colors = paletteFromTheme(m.activeTheme)

	m.applyEvent(agent.Event{
		Type:     agent.EventToolCall,
		ToolCall: &ai.ToolCall{ID: "t1", Name: "bash", Args: map[string]any{"command": "sleep 1"}},
	})
	if len(m.lines) != 1 || m.lines[0].kind != cardTool {
		t.Fatalf("want tool card, got %+v", m.lines)
	}
	if m.lines[0].startedAt.IsZero() {
		t.Fatal("startedAt should be set on tool call")
	}
	// Backdate so Elapsed shows a non-zero duration.
	m.lines[0].startedAt = time.Now().Add(-1500 * time.Millisecond)

	out := m.renderCardForTest(0, m.lines[0], 80)
	if !strings.Contains(out, "\x1b[48;") {
		t.Fatalf("EventToolCall render should include bg CSI, got %q", out)
	}
	if !strings.Contains(out, "Elapsed") {
		t.Fatalf("EventToolCall render should include Elapsed, got %q", out)
	}
}
