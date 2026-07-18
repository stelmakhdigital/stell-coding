package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"


	"stell/coding-agent/internal/agent"
	"stell/coding-agent/internal/config"
	"stell/coding-agent/internal/extensions"
	"stell/coding-agent/internal/packages"
	"stell/agent/session"
	"stell/coding-agent/internal/themes"
	"stell/coding-agent/internal/trust"
)

func (m *Model) handleSlash(input string) Cmd {
	line := strings.TrimSpace(strings.TrimPrefix(input, "/"))
	if line == "" {
		return nil
	}
	parts := strings.Fields(line)
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "help", "h", "?":
		m.addInfo(helpText)
		return nil
	case "hotkeys":
		if len(args) >= 1 {
			switch strings.ToLower(args[0]) {
			case "save":
				if err := SaveKeybindings(m.cfg.GlobalDir, m.keys, true); err != nil {
					m.addError(err.Error())
					return nil
				}
				m.addInfo("saved keybindings → " + KeybindingsPath(m.cfg.GlobalDir))
				return nil
			case "reset":
				m.keys = DefaultKeybindings()
				if m.svc.Extensions != nil {
					m.keys.MergeExtensionShortcuts(m.svc.Extensions.Shortcuts())
				}
				m.rebuildOverlayKeys()
				m.editorKeys = NewKeybindingsManager(DefaultTUIKeybindings(), nil)
				if err := SaveKeybindings(m.cfg.GlobalDir, m.keys, false); err != nil {
					m.addError(err.Error())
					return nil
				}
				m.addInfo("reset keybindings to defaults and saved")
				return nil
			}
		}
		m.addInfo(hotkeysText(m.keys))
		m.addInfo("hint: /hotkeys save | /hotkeys reset")
		return nil
	case "changelog":
		m.addInfo(changelogText)
		return nil
	case "copy":
		m.copyLastAssistant()
		return nil
	case "preview":
		if len(args) > 0 && strings.EqualFold(args[0], "external") {
			return m.openMarkdownPagerCmd()
		}
		m.openMarkdownPreview()
		m.syncViewport()
		return nil
	case "import":
		if len(args) < 1 {
			m.addInfo("usage: /import <path.jsonl>")
			return nil
		}
		if err := m.svc.OpenSession(args[0]); err != nil {
			m.addError(err.Error())
			return nil
		}
		m.lines = nil
		m.hydrateSession()
		m.addInfo("imported: " + args[0])
		m.syncViewport()
		return nil
	case "trust":
		ws := m.cfg.Workspace
		parent := len(args) > 0 && (args[0] == "parent" || args[0] == "--parent")
		if err := trust.TrustWorkspaceOpts(m.cfg.GlobalDir, ws, parent); err != nil {
			m.addError(err.Error())
			return nil
		}
		msg := "trusted workspace (restart to apply project settings): " + ws
		if parent {
			msg += " + parent"
		}
		m.addInfo(msg)
		return nil
	case "quit", "exit", "q":
		return QuitCmd()
	case "abort":
		restored := m.svc.DrainQueues()
		m.svc.Abort()
		if len(restored) > 0 {
			m.composer.SetValue(strings.Join(restored, "\n"))
			m.addInfo("aborted · restored queue")
		} else {
			m.addInfo("aborted")
		}
		return nil
	case "new":
		if _, err := m.svc.NewSession(m.ctx, ""); err != nil {
			m.addError(err.Error())
			return nil
		}
		m.lines = nil
		m.streaming = false
		m.streamBuf.Reset()
		m.showStartup = true
		m.addInfo("new session: " + m.svc.Sessions.Header.ID)
		m.hydrateSession()
		return nil
	case "tree":
		m.svc.EmitBeforeTree(m.ctx)
		m.openTreeOverlay()
		m.syncViewport()
		return nil
	case "resume":
		m.openSessionOverlay()
		m.syncViewport()
		return nil
	case "fork":
		if len(args) < 1 {
			m.addInfo("usage: /fork <entryId>")
			return nil
		}
		leaf, err := m.svc.ForkSession(args[0])
		if err != nil {
			m.addError(err.Error())
			return nil
		}
		m.hydrateSession()
		m.addInfo("forked at " + args[0] + " → leaf " + leaf)
		return nil
	case "clone":
		path, err := m.svc.CloneSession()
		if err != nil {
			m.addError(err.Error())
			return nil
		}
		m.lines = nil
		m.hydrateSession()
		m.addInfo("cloned → " + path)
		m.syncViewport()
		return nil
	case "export":
		dest := "session-export.html"
		format := "html"
		if len(args) >= 1 {
			dest = args[0]
			if strings.HasSuffix(strings.ToLower(dest), ".jsonl") {
				format = "jsonl"
			}
		}
		if format == "jsonl" {
			if err := m.svc.ExportSession(dest); err != nil {
				m.addError(err.Error())
				return nil
			}
			m.addInfo("exported → " + dest)
			return nil
		}
		path, err := m.svc.ExportSessionHTML(dest)
		if err != nil {
			m.addError(err.Error())
			return nil
		}
		m.addInfo("exported → " + path)
		return nil
	case "share":
		url, err := m.svc.ShareSessionGist()
		if err != nil {
			m.addError(err.Error())
			return nil
		}
		m.addInfo("shared → " + url)
		return nil
	case "compact":
		var info *agent.CompactInfo
		var err error
		if len(args) > 0 {
			info, err = m.svc.CompactWithInstructions(m.ctx, strings.Join(args, " "))
		} else {
			info, err = m.svc.Compact(m.ctx)
		}
		if err != nil {
			m.addError(err.Error())
			return nil
		}
		m.hydrateSession()
		m.addInfo(fmt.Sprintf("compact: removed %d messages · %s", info.RemovedMessages, info.SummaryPreview))
		return nil
	case "model", "models":
		if len(args) >= 1 {
			term := strings.Join(args, " ")
			if err := m.svc.SetModelByName(args[0]); err == nil {
				m.addInfo("model → " + args[0])
				return nil
			}
			m.openModelOverlayFiltered(term)
			m.syncViewport()
			return nil
		}
		m.openModelOverlay()
		m.syncViewport()
		return nil
	case "theme":
		if len(args) >= 1 {
			m.cfg.Settings.Theme = args[0]
			m.reloadTheme()
			m.addInfo("theme → " + args[0])
			return nil
		}
		m.openThemeOverlay()
		m.syncViewport()
		return nil
	case "themes":
		list, err := themes.Resolve(themes.ResolveOpts{
			GlobalDir:  m.cfg.GlobalDir,
			ProjectDir: m.cfg.ProjectDir,
			Workspace:  m.cfg.Workspace,
		})
		if err != nil || len(list) == 0 {
			m.addInfo("no themes found")
			return nil
		}
		for _, t := range list {
			cur := ""
			if t.Name == m.cfg.Settings.Theme || (m.cfg.Settings.Theme == "" && t.Name == "dark") {
				cur = " *"
			}
			src := "builtin"
			if t.Path() != "" {
				src = t.Path()
			}
			m.addInfo(t.Name + cur + "  (" + src + ")")
		}
		return nil
	case "session":
		return m.handleSessionSub(args)
	case "pkg":
		return m.handlePkgSub(args)
	case "state":
		st := m.svc.GetState()
		m.addInfo(fmt.Sprintf("streaming=%v messages=%d pending=%d",
			st.IsStreaming, st.MessageCount, st.PendingCount))
		return nil
	case "reload":
		st, err := m.svc.ReloadExtensions(m.ctx)
		if err != nil {
			m.addError(err.Error())
			return nil
		}
		for _, s := range st {
			if s.OK {
				m.addInfo(fmt.Sprintf("reloaded %s/%s", s.Package, s.Extension))
			} else {
				m.addError(fmt.Sprintf("%s/%s: %s", s.Package, s.Extension, s.Error))
			}
		}
		if len(st) == 0 {
			m.addInfo("reload complete (no extensions)")
		}
		m.keys = LoadKeybindingsWithExtensions(m.cfg.GlobalDir, m.svc.Extensions)
		m.rebuildOverlayKeys()
		m.editorKeys = NewKeybindingsManager(DefaultTUIKeybindings(), LoadUserKeyOverrides(m.cfg.GlobalDir))
		m.reloadTheme()
		return nil
	case "settings":
		m.openSettingsOverlay()
		m.syncViewport()
		return nil
	case "scoped-models", "scopedmodels":
		m.openScopedModelsOverlay()
		m.syncViewport()
		return nil
	case "login":
		return m.handleLoginCommand(args)
	case "logout":
		provider := "anthropic"
		if len(args) >= 1 {
			provider = args[0]
		}
		auth, err := config.LoadAuth(m.cfg.GlobalDir)
		if err != nil {
			m.addError(err.Error())
			return nil
		}
		if err := auth.Logout(provider); err != nil {
			m.addError(err.Error())
			return nil
		}
		m.addInfo("logged out " + provider)
		return nil
	case "commands":
		for _, c := range m.svc.ExtensionCommands() {
			m.addInfo(fmt.Sprintf("%s — %s (%s)", c.Name, c.Description, c.Source))
		}
		return nil
	case "skills":
		if m.svc.Catalog == nil || m.svc.Catalog.Skills == nil {
			m.addInfo("no skills loaded")
			return nil
		}
		for _, e := range m.svc.Catalog.Skills.List() {
			m.addInfo(fmt.Sprintf("%s — %s", e.Name, e.Description))
		}
		return nil
	case "skill":
		if len(args) < 1 {
			m.openSkillOverlay()
			m.syncViewport()
			return nil
		}
		name := args[0]
		msg := ""
		if len(args) > 1 {
			msg = strings.Join(args[1:], " ")
		}
		prefix := "/skill:" + name
		if msg != "" {
			prefix += " " + msg
		}
		return m.submitWithText(prefix, false)
	case "prompt":
		if len(args) < 1 {
			m.openPromptOverlay()
			m.syncViewport()
			return nil
		}
		prefix := "/" + args[0]
		if len(args) > 1 {
			prefix += " " + strings.Join(args[1:], " ")
		}
		return m.submitWithText(prefix, false)
	case "prompts":
		if m.svc.Catalog == nil || m.svc.Catalog.Prompts == nil {
			m.addInfo("no prompts loaded")
			return nil
		}
		for _, e := range m.svc.Catalog.Prompts.List() {
			m.addInfo(fmt.Sprintf("%s — %s", e.Name, e.Description))
		}
		return nil
	default:
		if m.svc.Extensions != nil {
			cmdName := "/" + cmd
			if owner := findExtCommand(m.svc.ExtensionCommands(), cmdName); owner {
				res, err := m.svc.InvokeExtensionCommand(m.ctx, cmdName, args)
				if err != nil {
					m.addError(err.Error())
					return nil
				}
				if res.Message != "" {
					m.addInfo(res.Message)
				}
				return nil
			}
		}
		m.addInfo("unknown command: /" + cmd)
		return nil
	}
}

func (m *Model) handleSessionSub(args []string) Cmd {
	if len(args) == 0 || strings.EqualFold(args[0], "info") {
		stats := m.svc.GetSessionStats()
		var b strings.Builder
		fmt.Fprintf(&b, "session %v\n", stats["sessionId"])
		fmt.Fprintf(&b, "file: %v\n", stats["sessionFile"])
		if n, _ := stats["sessionName"].(string); n != "" {
			b.WriteString("name: " + n + "\n")
		}
		fmt.Fprintf(&b, "messages: %v · model: %v\n", stats["messageCount"], stats["modelName"])
		if c, ok := stats["cost"]; ok {
			fmt.Fprintf(&b, "cost: %v\n", c)
		}
		if tok, ok := stats["tokens"].(map[string]any); ok {
			fmt.Fprintf(&b, "tokens: in=%v out=%v\n", tok["input"], tok["output"])
		}
		m.addInfo(strings.TrimSpace(b.String()))
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "list":
		files, err := m.svc.ListSessions()
		if err != nil {
			m.addError(err.Error())
			return nil
		}
		if len(files) == 0 {
			m.addInfo("no sessions")
			return nil
		}
		for _, f := range files {
			m.addInfo(fmt.Sprintf("%s  %s", f.ModTime.Format("2006-01-02 15:04"), filepath.Base(f.Path)))
		}
		return nil
	case "switch":
		if len(args) < 2 {
			m.addInfo("usage: /session switch <path>")
			return nil
		}
		if _, err := m.svc.SwitchSession(m.ctx, args[1]); err != nil {
			m.addError(err.Error())
			return nil
		}
		m.lines = nil
		m.hydrateSession()
		m.addInfo("switched → " + args[1])
		return nil
	case "name":
		if len(args) < 2 {
			m.addInfo("usage: /session name <label>")
			return nil
		}
		m.svc.SetSessionName(strings.Join(args[1:], " "))
		m.addInfo("session name → " + strings.Join(args[1:], " "))
		return nil
	default:
		m.addInfo("usage: /session [info]|list|switch|name")
		return nil
	}
}

func (m *Model) handlePkgSub(args []string) Cmd {
	if len(args) == 0 {
		m.addInfo("usage: /pkg list|remove|update [name]")
		return nil
	}
	mgr := packages.NewManager(m.cfg.GlobalDir, m.cfg.ProjectDir, "project")
	switch strings.ToLower(args[0]) {
	case "list":
		recs, err := mgr.List()
		if err != nil {
			m.addError(err.Error())
			return nil
		}
		if len(recs) == 0 {
			m.addInfo("no packages installed")
			return nil
		}
		for _, r := range recs {
			m.addInfo(fmt.Sprintf("%s@%s  %s", r.Name, r.Version, r.Source))
		}
		return nil
	case "remove":
		if len(args) < 2 {
			m.addInfo("usage: /pkg remove <name>")
			return nil
		}
		if err := mgr.Remove(args[1]); err != nil {
			m.addError(err.Error())
			return nil
		}
		m.addInfo("removed " + args[1])
		return nil
	case "update":
		name := ""
		if len(args) >= 2 {
			name = args[1]
		}
		if err := mgr.Update(context.Background(), name); err != nil {
			m.addError(err.Error())
			return nil
		}
		m.addInfo("package update complete")
		return nil
	default:
		m.addInfo("usage: /pkg list|remove|update [name]")
		return nil
	}
}

func (m *Model) openThemeOverlay() {
	list, err := themes.Resolve(themes.ResolveOpts{
		GlobalDir:  m.cfg.GlobalDir,
		ProjectDir: m.cfg.ProjectDir,
		Workspace:  m.cfg.Workspace,
	})
	if err != nil || len(list) == 0 {
		m.addInfo("no themes found")
		return
	}
	m.modelNames = make([]string, 0, len(list))
	allowed := m.cfg.Settings.Themes
	for _, t := range list {
		if len(allowed) > 0 && !containsStr(allowed, t.Name) {
			continue
		}
		m.modelNames = append(m.modelNames, t.Name)
	}
	if len(m.modelNames) == 0 {
		m.addInfo("no themes found")
		return
	}
	cursor := 0
	active := themes.ResolveThemeSetting(m.cfg.Settings.Theme, themes.DetectDefaultName())
	if active == "" {
		active = m.cfg.Settings.Theme
	}
	for i, name := range m.modelNames {
		if name == active {
			cursor = i
			break
		}
	}
	m.pushOverlayFrame(overlayFrame{
		mode:        overlayModel,
		listKind:    "theme",
		text:        renderThemeOverlay(m.modelNames, cursor, active),
		cursor:      cursor,
		modelNames:  append([]string(nil), m.modelNames...),
		overlayList: "theme",
	})
}

func renderThemeOverlay(names []string, cursor int, current string) string {
	var b strings.Builder
	b.WriteString("themes (↑/↓ select, enter apply, esc close)\n")
	for i, name := range names {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		line := prefix + name
		if name == current {
			line += " *"
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) openSkillOverlay() {
	if m.svc.Catalog == nil || m.svc.Catalog.Skills == nil {
		m.addInfo("no skills loaded")
		return
	}
	m.modelNames = nil
	for _, e := range m.svc.Catalog.Skills.List() {
		m.modelNames = append(m.modelNames, e.Name)
	}
	m.pushOverlayFrame(overlayFrame{
		mode:        overlayModel,
		listKind:    "skill",
		text:        renderListOverlay("skills", m.modelNames, 0),
		cursor:      0,
		modelNames:  append([]string(nil), m.modelNames...),
		overlayList: "skill",
	})
}

func (m *Model) openPromptOverlay() {
	if m.svc.Catalog == nil || m.svc.Catalog.Prompts == nil {
		m.addInfo("no prompts loaded")
		return
	}
	m.modelNames = nil
	for _, e := range m.svc.Catalog.Prompts.List() {
		m.modelNames = append(m.modelNames, e.Name)
	}
	m.pushOverlayFrame(overlayFrame{
		mode:        overlayModel,
		listKind:    "prompt",
		text:        renderListOverlay("prompts", m.modelNames, 0),
		cursor:      0,
		modelNames:  append([]string(nil), m.modelNames...),
		overlayList: "prompt",
	})
}

func renderListOverlay(title string, names []string, cursor int) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString(" (↑/↓ select, enter, esc close)\n")
	for i, name := range names {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		b.WriteString(prefix)
		b.WriteString(name)
		b.WriteString("\n")
	}
	return b.String()
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func findExtCommand(cmds []extensions.CommandEntry, name string) bool {
	for _, c := range cmds {
		if c.Name == name || c.Name == strings.TrimPrefix(name, "/") {
			return true
		}
	}
	return false
}

const helpText = `slash commands:
  /help     this help
  /hotkeys  keyboard shortcuts
  /changelog version notes
  /quit     exit
  /new      new session
  /resume   session picker
  /tree     session tree
  /fork     fork at entry id
  /clone    clone session to new file
  /export   export session html|jsonl
  /import   import session jsonl
  /copy     copy last assistant message
  /preview  markdown preview ([external] pager)
  /share    share session as private gist
  /abort    cancel streaming run (restores queue)
  /commands list extension slash commands
  /compact  compact conversation history
  /model    switch model (picker or /model <name>)
  /scoped-models  scoped model list
  /theme    switch theme
  /themes   list themes
  /session  info|list|switch|name
  /trust    save project trust decision
  /settings settings overlay
  /login    OAuth login ([provider])
  /logout   clear auth (optional provider)
  /reload   reload extensions, keybindings, theme
  /skills   list skills
  /skill    pick skill (/skill:name)
  /prompts  list prompt templates
  /prompt   alias for /name (picker or /prompt hello …)

composer:
  /name     expand prompt template (e.g. /hello world)
  /skill:name  load skill into message
  !cmd      run shell (output sent to model)
  !!cmd     run shell (output not sent to model)
  @query    file picker (inline while typing)

keys:
  enter         send (steer while streaming)
  alt+enter     follow-up while streaming
  alt+up        restore queued message
  esc           interrupt / close overlay
  ctrl+c        clear editor (twice to quit)
  ctrl+d        delete-forward
  ctrl+z        suspend
  ctrl+g        external editor
  ctrl+shift+v  markdown preview
  ctrl+p        cycle model
  ctrl+l        model picker
  ctrl+t        toggle thinking blocks
  ctrl+x        copy last assistant
  ctrl+shift+m  scoped models
  shift+tab     cycle thinking level
  /tree         session tree
  pgup/pgdown   scroll`

const changelogText = `stell coding agent
- Differential TUI (Component tree + CSI 2026)
- Steer / follow-up queues with pending strip
- Extensions UI: widgets, replace editor, working indicator
- Session tree, export/share, themes (51 tokens)
See docs/RU for details.`

func hotkeysText(keys Keybindings) string {
	var b strings.Builder
	b.WriteString("keyboard shortcuts:\n")
	order := []string{
		actionSubmit, actionFollowUp, actionInterrupt, actionClear, actionDeleteForward, actionSuspend,
		actionMessageDequeue, actionModelCycle, actionModelCycleBack, actionModelSelect,
		actionTreeOpen, actionThinkingCycle, actionThinkingToggle,
		actionExternalEditor, actionMarkdownPreview, actionMessageCopy, actionScopedModels, actionPasteClipboard,
		actionScrollUp, actionScrollDown, actionSettingsOpen, actionEditorYank,
		actionEditorPageUp, actionEditorPageDown, actionTreeEditLabel, actionTreeFold, actionTreeUnfold,
		actionSessionRename, actionSessionDelete, actionSessionToggleSort, actionSessionToggleNamed,
		actionTreeFilterDefault, actionTreeFilterNoTools, actionTreeFilterUserOnly,
		actionTreeFilterLabeled, actionTreeFilterAll, actionTreeFilterCycle, actionTreeFilterCycleBack,
		actionCardFocus, actionCardUp, actionCardDown,
	}
	for _, a := range order {
		k := KeyDisplay(keys, a)
		if k == "" {
			continue
		}
		fmt.Fprintf(&b, "  %-14s  %s\n", k, a)
	}
	b.WriteString("\noverlay notes:\n")
	b.WriteString("  tree: enter navigate · shift+f fork · f filter cycle\n")
	b.WriteString("  session: enter open · ctrl+r rename · ctrl+d delete\n")
	b.WriteString("  ctrl+d in editor: delete-forward")
	return strings.TrimSpace(b.String())
}

func (m *Model) handleLoginCommand(args []string) Cmd {
	providers := []string{"anthropic", "openai", "github-copilot", "radius"}
	if len(args) == 0 {
		m.addInfo("OAuth providers: " + strings.Join(providers, ", ") +
			"\n  /login <provider>     → show device-login command" +
			"\n  API key: exit TUI and run: stell login <provider>")
		return nil
	}
	provider := strings.ToLower(args[0])
	ok := false
	for _, p := range providers {
		if p == provider {
			ok = true
			break
		}
	}
	if !ok {
		m.addInfo("unknown provider " + provider + "; try: " + strings.Join(providers, ", "))
		return nil
	}
	m.addInfo(fmt.Sprintf("in another terminal run:\n  stell login --device %s\nthen /reload here", provider))
	return nil
}

func renderTree(sess *session.Manager) string {
	if len(sess.Entries) == 0 {
		return "session tree: (empty)"
	}
	items := buildTreeItems(sess, "all", "", nil)
	if len(items) == 0 {
		return "session tree: (empty)"
	}
	var b strings.Builder
	b.WriteString("session tree\n")
	for _, it := range items {
		indent := ""
		if it.depth > 0 {
			indent = strings.Repeat("  ", it.depth-1) + "├─ "
		}
		b.WriteString(indent)
		b.WriteString(it.label)
		b.WriteString("\n")
	}
	return b.String()
}
