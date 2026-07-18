package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stelmakhdigital/stell-coding/internal/agent"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-agent/tools"
)

type bashProgressMsg struct {
	output string
}

type bashDoneMsg struct {
	command string
	exclude bool
	result  tools.BashResult
	err     error
}

// parseUserBashInput разбирает bash из композера: !cmd (в контексте) или !!cmd (исключён).
func parseUserBashInput(text string) (command string, excludeFromContext bool, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "!") {
		return "", false, false
	}
	if strings.HasPrefix(text, "!!") {
		cmd := strings.TrimSpace(strings.TrimPrefix(text, "!!"))
		return cmd, true, cmd != ""
	}
	cmd := strings.TrimSpace(strings.TrimPrefix(text, "!"))
	return cmd, false, cmd != ""
}

func formatBashCard(command, output string, exclude bool, res tools.BashResult) string {
	prefix := "$ "
	if exclude {
		prefix = "$ !! "
	}
	body := prefix + command
	if output != "" {
		body += "\n" + output
	}
	if res.Cancelled {
		body += "\n(cancelled)"
	} else if res.ExitCode != 0 && !res.Cancelled {
		body += fmt.Sprintf("\n[exit %d]", res.ExitCode)
	}
	return body
}

func formatBashEntryCard(e session.Entry) string {
	if e.Message == nil {
		return ""
	}
	meta := session.EntryBashMeta(e)
	content := e.Message.Content
	if meta.ExcludeFromContext && strings.HasPrefix(content, "$ ") {
		content = "$ !! " + strings.TrimPrefix(content, "$ ")
	}
	if meta.Cancelled {
		content += "\n(cancelled)"
	} else if meta.ExitCode != 0 {
		content += fmt.Sprintf("\n[exit %d]", meta.ExitCode)
	}
	return content
}

func (m *Model) updateBashMode() {
	m.bashMode = strings.HasPrefix(strings.TrimSpace(m.composer.Value()), "!")
}

func (m *Model) startUserBash(command string, exclude bool) Cmd {
	m.bashStreamIdx = len(m.lines)
	now := time.Now()
	m.lines = append(m.lines, card{
		kind:        cardBash,
		body:        formatBashCard(command, "", exclude, tools.BashResult{}),
		toolName:    "bash",
		toolPath:    command,
		status:      cardStatusPending,
		startedAt:   now,
		excludeBash: exclude,
	})
	m.syncViewport()

	ch := make(chan Msg, 16)
	m.bashCh = ch
	go func() {
		res, err := m.svc.RunBash(m.ctx, command, agent.RunBashOptions{
			ExcludeFromContext: exclude,
			OnProgress: func(partial string) {
				select {
				case ch <- bashProgressMsg{output: partial}:
				default:
				}
			},
		})
		ch <- bashDoneMsg{command: command, exclude: exclude, result: res, err: err}
		close(ch)
	}()
	return m.waitBash(ch)
}

func (m *Model) waitBash(ch <-chan Msg) Cmd {
	return func() Msg {
		msg, ok := <-ch
		if !ok {
			m.bashCh = nil
			return nil
		}
		return msg
	}
}

func (m *Model) handleBashProgress(msg bashProgressMsg) Cmd {
	if m.bashStreamIdx >= 0 && m.bashStreamIdx < len(m.lines) && m.lines[m.bashStreamIdx].kind == cardBash {
		c := &m.lines[m.bashStreamIdx]
		body := stripRunningHeartbeat(msg.output)
		c.toolContent = body
		c.body = formatBashCard(c.toolPath, body, c.excludeBash, tools.BashResult{})
		c.status = cardStatusPending
		m.syncViewport()
	}
	if m.bashCh != nil {
		return m.waitBash(m.bashCh)
	}
	return nil
}

func (m *Model) handleBashDone(msg bashDoneMsg) Cmd {
	idx := m.bashStreamIdx
	m.bashStreamIdx = -1
	m.bashCh = nil
	if msg.err != nil {
		if idx >= 0 && idx < len(m.lines) && m.lines[idx].kind == cardBash {
			m.lines = append(m.lines[:idx], m.lines[idx+1:]...)
		}
		if msg.err == agent.ErrBashRunning {
			m.addInfo("A bash command is already running. Press Esc to cancel it first.")
		} else {
			m.addError(msg.err.Error())
		}
		m.syncViewport()
		return nil
	}
	if idx >= 0 && idx < len(m.lines) && m.lines[idx].kind == cardBash {
		c := &m.lines[idx]
		out := stripRunningHeartbeat(msg.result.Output)
		c.body = formatBashCard(msg.command, out, msg.exclude, msg.result)
		c.toolContent = out
		if msg.result.Cancelled || msg.result.ExitCode != 0 {
			c.status = cardStatusError
		} else {
			c.status = cardStatusSuccess
		}
		c.endedAt = time.Now()
		if c.startedAt.IsZero() {
			c.startedAt = c.endedAt
		}
	}
	m.syncViewport()
	return nil
}

func listWorkspaceFiles(workspace, query string) ([]string, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	var out []string
	err := filepath.WalkDir(workspace, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil || strings.HasPrefix(rel, ".git/") {
			return nil
		}
		if query == "" || strings.Contains(strings.ToLower(rel), query) {
			out = append(out, rel)
		}
		if len(out) >= 50 {
			return filepath.SkipAll
		}
		return nil
	})
	return out, err
}
