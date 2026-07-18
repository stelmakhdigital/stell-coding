package tui

import (
	"context"
	"fmt"
	"strings"

	"stell/coding-agent/internal/packages"
	"stell/coding-agent/internal/update"
	"stell/coding-agent/internal/version"
)

const changelogURL = "https://stell.dev/changelog"

type versionCheckMsg struct {
	release *update.LatestRelease
}

type packageUpdatesMsg struct {
	names []string
}

func (m *Model) checkLatestVersion() Cmd {
	return func() Msg {
		if update.SkipVersionCheck() {
			return nil
		}
		release, err := update.CheckForNewRelease(context.Background(), version.Version)
		if err != nil || release == nil {
			return nil
		}
		return versionCheckMsg{release: release}
	}
}

func (m *Model) checkPackageUpdates() Cmd {
	return func() Msg {
		if update.Offline() || m.cfg == nil {
			return nil
		}
		updates, err := packages.CheckAllScopes(context.Background(), m.cfg.GlobalDir, m.cfg.ProjectDir)
		if err != nil || len(updates) == 0 {
			return nil
		}
		names := make([]string, len(updates))
		for i, u := range updates {
			names[i] = u.DisplayName
		}
		return packageUpdatesMsg{names: names}
	}
}

func (m *Model) showNewVersionNotification(release update.LatestRelease) {
	body := fmt.Sprintf("New version %s is available. Run stell update\nChangelog: %s", release.Version, changelogURL)
	if note := strings.TrimSpace(release.Note); note != "" {
		body = note + "\n\n" + body
	}
	m.addWarningNotice("Update Available", body)
}

func (m *Model) showPackageUpdateNotification(names []string) {
	var b strings.Builder
	b.WriteString("Package updates are available. Run stell update --extensions\nPackages:")
	for _, n := range names {
		b.WriteString("\n- ")
		b.WriteString(n)
	}
	m.addWarningNotice("Package Updates Available", b.String())
}

func (m *Model) addWarningNotice(title, body string) {
	m.startupNotices = append(m.startupNotices, card{
		kind:      cardWarning,
		skillName: title,
		body:      body,
	})
	m.syncViewport()
}

func formatWarningCard(m *Model, title, body string, width int) string {
	p := m.colors
	p.Border = warnColor(m.colors)
	lines := renderDynamicBorder(body, title, "", width, p)
	return strings.Join(lines, "\n")
}
