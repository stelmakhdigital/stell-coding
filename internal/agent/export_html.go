package agent

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"strings"

	"github.com/stelmakhdigital/ai"
	"stell/agent/session"
	"stell/agent/tools"
)

// ExportSessionHTML записывает HTML-экспорт активной ветки.
func (s *Service) ExportSessionHTML(dest string) (string, error) {
	abs, _, err := tools.ResolveOutputPath(s.Config.Workspace, dest, "session-export.html")
	if err != nil {
		return "", err
	}
	branch := s.Sessions.ActiveBranch()
	stats := s.GetSessionStats()
	var b strings.Builder
	pageBg, cardBg := "#18181e", "#1e1e24"
	b.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>stell session</title>")
	b.WriteString("<style>")
	fmt.Fprintf(&b, "body{font-family:system-ui,sans-serif;margin:0;background:%s;color:#eee;padding:1.5rem}", pageBg)
	fmt.Fprintf(&b, ".entry{margin:1rem 0;padding:1rem;border-radius:8px;background:%s}", cardBg)
	b.WriteString(".user{border-left:4px solid #4af}.assistant{border-left:4px solid #aaa}")
	b.WriteString(".tool{border-left:4px solid #fa4}.meta{opacity:.7;margin-bottom:1rem}")
	b.WriteString("pre{white-space:pre-wrap;word-break:break-word}</style></head><body>")
	b.WriteString("<h1>Session export</h1><div class=\"meta\">")
	fmt.Fprintf(&b, "Session: %s<br>Model: %s</div>",
		html.EscapeString(fmt.Sprint(stats["sessionId"])),
		html.EscapeString(fmt.Sprint(stats["modelName"])))
	for _, e := range branch {
		writeHTMLEntry(&b, e)
	}
	b.WriteString("</body></html>")
	if err := os.WriteFile(abs, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return abs, nil
}

func writeHTMLEntry(b *strings.Builder, e session.Entry) {
	if e.Message == nil {
		return
	}
	cls := "entry"
	switch e.Message.Role {
	case ai.RoleUser:
		cls += " user"
	case ai.RoleAssistant:
		cls += " assistant"
	default:
		if ai.IsToolRole(e.Message.Role) || e.Message.Role == ai.RoleBashExecution {
			cls += " tool"
		}
	}
	fmt.Fprintf(b, `<div class="%s"><strong>%s</strong><pre>%s</pre></div>`,
		cls, html.EscapeString(string(e.Message.Role)), html.EscapeString(e.Message.Content))
}

// ShareSessionGist создаёт приватный GitHub gist через `gh` и возвращает URL.
func (s *Service) ShareSessionGist() (string, error) {
	path, err := s.ExportSessionHTML("session-share.html")
	if err != nil {
		return "", err
	}
	cmd := exec.Command("gh", "gist", "create", "--private", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh gist create: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return firstURL(string(out)), nil
}

func firstURL(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return strings.TrimSpace(s)
}
