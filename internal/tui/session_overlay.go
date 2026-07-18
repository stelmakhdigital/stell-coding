package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type sessionItem struct {
	path    string
	label   string
	modTime time.Time
}

func (m *Model) openSessionOverlay() {
	files, err := m.svc.ListSessions()
	if err != nil {
		m.addError(err.Error())
		return
	}
	items := make([]sessionItem, 0, len(files))
	for _, f := range files {
		label := filepath.Base(f.Path)
		if f.CWD != "" {
			label += " · " + trimPath(f.CWD, 30)
		}
		items = append(items, sessionItem{path: f.Path, label: label, modTime: f.ModTime})
	}
	items = m.sortSessionItems(items)
	if len(items) == 0 {
		m.addInfo("no sessions found")
		return
	}
	m.pushOverlayFrame(overlayFrame{
		mode:         overlaySession,
		text:         renderSessionOverlay(items, 0, m.sessionSortDesc, m.sessionNamedOnly),
		cursor:       0,
		sessionItems: items,
	})
}

func relativeSessionAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 48*time.Hour:
		return "1d"
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func renderSessionOverlay(items []sessionItem, cursor int, sortDesc, namedOnly bool) string {
	var b strings.Builder
	sortHint := "asc"
	if sortDesc {
		sortHint = "desc"
	}
	fmt.Fprintf(&b, "sort:%s · named:%v\n", sortHint, namedOnly)
	for i, it := range items {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		fmt.Fprintf(&b, "%s%s  %s\n", prefix, relativeSessionAge(it.modTime), it.label)
	}
	return b.String()
}

func (m *Model) switchSessionFromOverlay() {
	if len(m.sessionItems) == 0 {
		m.closeOverlay()
		return
	}
	path := m.sessionItems[m.overlayCursor].path
	if _, err := m.svc.SwitchSession(m.ctx, path); err != nil {
		m.addError(err.Error())
	} else {
		m.lines = nil
		m.hydrateSession()
		m.addInfo("opened → " + filepath.Base(path))
	}
	m.closeOverlay()
}

func (m *Model) confirmDeleteSession() {
	if len(m.sessionItems) == 0 {
		return
	}
	path := m.sessionItems[m.overlayCursor].path
	m.pushOverlayFrame(overlayFrame{
		mode: overlayConfirm,
		text: fmt.Sprintf("Delete session %s?\n\n  y — delete\n  n / esc — cancel\n", filepath.Base(path)),
	})
	m.pendingDeleteSession = path
}

func (m *Model) forkSessionFromOverlay() {
	if len(m.sessionItems) == 0 {
		return
	}
	path := m.sessionItems[m.overlayCursor].path
	if _, err := m.svc.SwitchSession(m.ctx, path); err != nil {
		m.addError(err.Error())
		return
	}
	m.lines = nil
	m.hydrateSession()
	m.addInfo("opened for fork → " + filepath.Base(path))
	m.closeOverlay()
	m.openTreeOverlay()
}
