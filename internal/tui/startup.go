package tui

import (
	"fmt"
	"strings"

	"stell/coding-agent/internal/version"
)

func (m Model) startupBanner() string {
	if m.cfg != nil && m.cfg.Settings.QuietStartupEnabled() {
		return ""
	}
	st := m.svc.GetState()
	var b strings.Builder
	b.WriteString(m.startupLogoLine())
	b.WriteByte('\n')
	base := "esc interrupt · ctrl+c clear · ctrl+d delete-forward · / commands · /hotkeys\n" +
		"enter send · alt+enter follow-up · @ files · ! shell"
	b.WriteString(m.colors.muted().Render(base))
	b.WriteString("\n\n")
	b.WriteString(m.colors.muted().Render(
		"/resume session picker · /tree navigate (enter) / fork (shift+f)\n" +
			"/model [term] picker · /scoped-models · /settings · /share · /abort\n" +
			"shift+tab thinking level · ctrl+t thinking blocks · ctrl+l models"))

	if st.ModelName != "" {
		b.WriteString("\n\n")
		b.WriteString(m.colors.muted().Render("model: " + st.ModelName))
	}

	if block := m.resourceStartupBlock(); block != "" {
		b.WriteString("\n\n")
		b.WriteString(block)
	}
	return b.String()
}

func (m Model) startupLogoLine() string {
	name := "stell"
	if m.extTitle != "" {
		name = m.extTitle
	}
	return m.colors.accent().Render(name) + m.colors.muted().Render(" v"+version.Version)
}

func (m Model) resourceStartupBlock() string {
	var b strings.Builder
	if names := m.loadedSkillNames(); len(names) > 0 {
		b.WriteString(m.colors.header().Render("[Skills]"))
		b.WriteByte('\n')
		b.WriteString(m.colors.muted().Render(strings.Join(names, ", ")))
		b.WriteByte('\n')
	}
	if names := m.loadedPromptNames(); len(names) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.colors.header().Render("[Prompts]"))
		b.WriteByte('\n')
		b.WriteString(m.colors.muted().Render(strings.Join(names, ", ")))
		b.WriteByte('\n')
	}
	if m.svc.Extensions != nil {
		if n := len(m.svc.ExtensionCommands()); n > 0 {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(m.colors.header().Render("[Extensions]"))
			b.WriteByte('\n')
			b.WriteString(m.colors.muted().Render(fmt.Sprintf("%d commands", n)))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (m Model) loadedSkillNames() []string {
	if m.svc.Catalog == nil || m.svc.Catalog.Skills == nil {
		return nil
	}
	list := m.svc.Catalog.Skills.List()
	out := make([]string, len(list))
	for i, e := range list {
		out[i] = e.Name
	}
	return out
}

func (m Model) loadedPromptNames() []string {
	if m.svc.Catalog == nil || m.svc.Catalog.Prompts == nil {
		return nil
	}
	list := m.svc.Catalog.Prompts.List()
	out := make([]string, len(list))
	for i, e := range list {
		out[i] = "/" + e.Name
	}
	return out
}

