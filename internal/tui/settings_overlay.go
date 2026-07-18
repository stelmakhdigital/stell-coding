package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/stelmakhdigital/stell-ai/provider/codex"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-coding/internal/telemetry"
)

func transportVal(s config.Settings) string {
	t := strings.TrimSpace(s.CodexTransport)
	if t == "" {
		return "auto"
	}
	return t
}

func telemetryEnsureTracking(s *config.Settings) bool {
	return telemetry.EnsureTrackingID(s)
}

func (m *Model) openSettingsOverlay() {
	items := m.buildSettingsListItems()
	accent := colorSeq(m.colors.Accent)
	muted := colorSeq(m.colors.Muted)
	// OnCancel nil: Esc закрывает через handleSettingsOverlayKey на текущей копии Update
	// (value-receiver Update делает closure момента открытия устаревшими).
	list := NewSettingsList(items, func(id, value string) {
		m.applySettingValue(id, value)
		m.syncOverlayFromComp()
	}, nil)
	list.Accent = accent
	list.Muted = muted
	m.pushOverlayFrame(overlayFrame{
		mode:     overlaySettings,
		listKind: "settings",
		comp:     list,
		anchor:   overlayAnchorCenter,
		maxHeightPct: 80,
	})
	m.syncOverlayFromComp()
}

func (m *Model) buildSettingsListItems() []SettingsItem {
	s := m.cfg.Settings
	themes := []string{"dark", "light"}
	if m.cfg != nil {
		// Текущая тема остаётся в списке переключения.
		if s.Theme != "" && s.Theme != "dark" && s.Theme != "light" {
			themes = append([]string{s.Theme}, themes...)
		}
	}
	thinking := []string{"off", "low", "medium", "high", "xhigh", "max"}
	modes := []string{"one-at-a-time", "all"}
	boolVals := []string{"true", "false"}
	models := []string{}
	for _, mod := range m.cfg.Models {
		models = append(models, mod.Name)
	}
	if len(models) == 0 {
		models = []string{s.DefaultModel}
	}
	treeFilters := []string{"default", "noTools", "userOnly", "labeledOnly", "all"}
	httpTimeouts := []string{"0", "60000", "300000", "600000"}
	return []SettingsItem{
		{ID: "defaultModel", Label: "defaultModel", Description: "Default model name", CurrentValue: s.DefaultModel, Values: models},
		{ID: "theme", Label: "theme", Description: "Theme name", CurrentValue: s.Theme, Values: themes},
		{ID: "defaultThinkingLevel", Label: "defaultThinkingLevel", Description: "off|low|medium|high|xhigh|max", CurrentValue: s.DefaultThinkingLevel, Values: thinking},
		{ID: "compaction.enabled", Label: "compaction.enabled", Description: "Auto-compact when context is full", CurrentValue: settingValue(s, "compaction.enabled"), Values: boolVals},
		{ID: "steeringMode", Label: "steeringMode", Description: "How steer messages are drained", CurrentValue: orDefault(s.SteeringMode, "one-at-a-time"), Values: modes},
		{ID: "followUpMode", Label: "followUpMode", Description: "How follow-up messages are drained", CurrentValue: orDefault(s.FollowUpMode, "one-at-a-time"), Values: modes},
		{ID: "externalEditor", Label: "externalEditor", Description: "Ctrl+G editor (cycle common editors)", CurrentValue: orDefault(s.ExternalEditor, s.ExternalEditorCommand()), Values: []string{"code --wait", "vim", "nvim", "nano", "emacs"}},
		{ID: "markdownPager", Label: "markdownPager", Description: "External markdown preview pager", CurrentValue: orDefault(s.MarkdownPager, s.MarkdownPagerCommand()), Values: []string{"less -R", "less", "more", "bat --paging=always"}},
		{ID: "showImages", Label: "showImages", Description: "Inline terminal images (Kitty/iTerm)", CurrentValue: boolStr(s.ImagesEnabled()), Values: boolVals},
		{ID: "showHardwareCursor", Label: "showHardwareCursor", Description: "Hardware cursor via CursorMarker", CurrentValue: boolStr(s.HardwareCursorEnabled()), Values: boolVals},
		{ID: "hideThinkingBlock", Label: "hideThinkingBlock", Description: "Hide thinking cards in chat", CurrentValue: boolStr(s.HideThinking()), Values: boolVals},
		{ID: "diffScroll", Label: "diffScroll", Description: "viewport scroll DiffEngine", CurrentValue: boolStr(s.DiffScrollEnabled()), Values: boolVals},
		{ID: "autoResizeImages", Label: "autoResizeImages", Description: "Scale images to width/cell budget", CurrentValue: boolStr(s.AutoResizeImagesEnabled()), Values: boolVals},
		{ID: "editorPaddingX", Label: "editorPaddingX", Description: "Left padding for editor body", CurrentValue: fmt.Sprintf("%d", s.EditorPaddingX), Values: []string{"0", "1", "2", "4"}},
		{ID: "doubleEscapeAction", Label: "doubleEscapeAction", Description: "Second Esc when idle", CurrentValue: s.DoubleEscapeActionOrDefault(), Values: []string{"tree", "fork", "none"}},
		{ID: "clearOnShrink", Label: "clearOnShrink", Description: "Force full redraw when terminal shrinks", CurrentValue: boolStr(s.ClearOnShrinkEnabled()), Values: boolVals},
		{ID: "enableInstallTelemetry", Label: "enableInstallTelemetry", Description: "Anonymous install/update ping", CurrentValue: boolStr(s.InstallTelemetryEnabled()), Values: boolVals},
		{ID: "enableAnalytics", Label: "enableAnalytics", Description: "Opt-in analytics", CurrentValue: boolStr(s.AnalyticsEnabled()), Values: boolVals},
		{ID: "quietStartup", Label: "quietStartup", Description: "Reduce startup banner", CurrentValue: boolStr(s.QuietStartupEnabled()), Values: boolVals},
		{ID: "collapseChangelog", Label: "collapseChangelog", Description: "Collapse changelog on startup", CurrentValue: boolStr(s.CollapseChangelogEnabled()), Values: boolVals},
		{ID: "transport", Label: "transport", Description: "Codex transport auto|sse|websocket", CurrentValue: transportVal(s), Values: []string{"auto", "sse", "websocket"}},
		{ID: "treeFilterMode", Label: "treeFilterMode", Description: "Default /tree filter", CurrentValue: s.TreeFilterModeOrDefault(), Values: treeFilters},
		{ID: "outputPad", Label: "outputPad", Description: "Horizontal padding for messages (0|1)", CurrentValue: fmt.Sprintf("%d", s.OutputPadOrDefault()), Values: []string{"0", "1"}},
		{ID: "autocompleteMaxVisible", Label: "autocompleteMaxVisible", Description: "Autocomplete dropdown rows (3-20)", CurrentValue: fmt.Sprintf("%d", s.AutocompleteMaxVisibleOrDefault()), Values: []string{"3", "5", "8", "10", "15", "20"}},
		{ID: "blockImages", Label: "blockImages", Description: "Block images sent to LLM", CurrentValue: boolStr(s.BlockImagesEnabled()), Values: boolVals},
		{ID: "showTerminalProgress", Label: "showTerminalProgress", Description: "OSC 9;4 terminal progress", CurrentValue: boolStr(s.ShowTerminalProgressEnabled()), Values: boolVals},
		{ID: "httpIdleTimeoutMs", Label: "httpIdleTimeoutMs", Description: "HTTP idle timeout ms (0=off)", CurrentValue: fmt.Sprintf("%d", s.HTTPIdleTimeoutOrDefault()), Values: httpTimeouts},
		{ID: "websocketConnectTimeoutMs", Label: "websocketConnectTimeoutMs", Description: "WS connect timeout ms", CurrentValue: fmt.Sprintf("%d", s.WebsocketConnectTimeoutOrDefault()), Values: []string{"0", "15000", "30000", "60000"}},
		{ID: "branchSummary.reserveTokens", Label: "branchSummary.reserveTokens", Description: "Branch summary reserve tokens", CurrentValue: fmt.Sprintf("%d", s.BranchSummaryReserveTokensOrDefault()), Values: []string{"8192", "16384", "32768"}},
		{ID: "branchSummary.skipPrompt", Label: "branchSummary.skipPrompt", Description: "Skip branch summary prompt on /tree", CurrentValue: boolStr(s.BranchSummarySkipPrompt()), Values: boolVals},
		{ID: "retry.provider.maxRetries", Label: "retry.provider.maxRetries", Description: "Provider SDK retries (0=off)", CurrentValue: fmt.Sprintf("%d", s.Retry.Provider.MaxRetries), Values: []string{"0", "1", "2", "3"}},
	}
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func settingValue(s config.Settings, key string) string {
	switch key {
	case "defaultModel":
		return s.DefaultModel
	case "externalEditor":
		if s.ExternalEditor != "" {
			return s.ExternalEditor
		}
		return s.ExternalEditorCommand()
	case "markdownPager":
		if s.MarkdownPager != "" {
			return s.MarkdownPager
		}
		return s.MarkdownPagerCommand()
	case "theme":
		return s.Theme
	case "defaultThinkingLevel":
		return s.DefaultThinkingLevel
	case "compaction.enabled":
		if s.Compaction.Enabled == nil || *s.Compaction.Enabled {
			return "true"
		}
		return "false"
	case "steeringMode":
		return orDefault(s.SteeringMode, "one-at-a-time")
	case "followUpMode":
		return orDefault(s.FollowUpMode, "one-at-a-time")
	default:
		return ""
	}
}

func (m *Model) applySettingValue(id, value string) {
	s := &m.cfg.Settings
	switch id {
	case "defaultModel":
		s.DefaultModel = value
	case "theme":
		s.Theme = value
		m.reloadTheme()
	case "defaultThinkingLevel":
		s.DefaultThinkingLevel = value
		m.svc.SetThinkingLevel(value)
	case "compaction.enabled":
		en := value == "true"
		s.Compaction.Enabled = &en
		m.svc.SetAutoCompaction(en)
	case "steeringMode":
		s.SteeringMode = value
		m.svc.SetSteeringMode(value)
	case "followUpMode":
		s.FollowUpMode = value
		m.svc.SetFollowUpMode(value)
	case "externalEditor":
		s.ExternalEditor = value
	case "markdownPager":
		s.MarkdownPager = value
	case "showImages":
		en := value == "true"
		s.ShowImages = &en
	case "showHardwareCursor":
		en := value == "true"
		s.ShowHardwareCursor = &en
		if m.composer.ed != nil {
			m.composer.ed.SetShowHardwareCursor(en)
		}
	case "hideThinkingBlock":
		en := value == "true"
		s.HideThinkingBlock = &en
	case "diffScroll":
		en := value == "true"
		s.DiffScroll = &en
	case "autoResizeImages":
		en := value == "true"
		s.AutoResizeImages = &en
	case "editorPaddingX":
		n := 0
		_, _ = fmt.Sscanf(value, "%d", &n)
		s.EditorPaddingX = n
		if m.composer.ed != nil {
			m.composer.ed.SetPaddingX(n)
		}
	case "doubleEscapeAction":
		s.DoubleEscapeAction = value
	case "clearOnShrink":
		en := value == "true"
		s.ClearOnShrink = &en
	case "enableInstallTelemetry":
		en := value == "true"
		s.EnableInstallTelemetry = &en
	case "enableAnalytics":
		en := value == "true"
		s.EnableAnalytics = &en
		if en {
			_ = telemetryEnsureTracking(s)
		}
	case "quietStartup":
		en := value == "true"
		s.QuietStartup = &en
	case "collapseChangelog":
		en := value == "true"
		s.CollapseChangelog = &en
	case "transport":
		s.CodexTransport = value
		codex.SetDefaultTransport(value)
	case "treeFilterMode":
		s.TreeFilterMode = config.NormalizeTreeFilterMode(value)
		m.treeFilter = s.TreeFilterModeOrDefault()
	case "outputPad":
		if value == "0" {
			s.OutputPad = 0
		} else {
			s.OutputPad = 1
		}
	case "autocompleteMaxVisible":
		if n, err := strconv.Atoi(value); err == nil {
			s.AutocompleteMaxVisible = n
		}
	case "blockImages":
		en := value == "true"
		s.BlockImages = &en
	case "showTerminalProgress":
		en := value == "true"
		s.ShowTerminalProgress = &en
	case "httpIdleTimeoutMs":
		if n, err := strconv.Atoi(value); err == nil {
			s.HTTPIdleTimeoutMs = n
		}
	case "websocketConnectTimeoutMs":
		if n, err := strconv.Atoi(value); err == nil {
			s.WebsocketConnectTimeoutMs = n
		}
	case "branchSummary.reserveTokens":
		if n, err := strconv.Atoi(value); err == nil {
			s.BranchSummary.ReserveTokens = n
		}
	case "branchSummary.skipPrompt":
		en := value == "true"
		s.BranchSummary.SkipPrompt = &en
	case "retry.provider.maxRetries":
		if n, err := strconv.Atoi(value); err == nil {
			s.Retry.Provider.MaxRetries = n
		}
	}
	if err := config.SaveGlobalSettings(m.cfg.GlobalDir, *s); err != nil {
		m.addError("settings save: " + err.Error())
		return
	}
	m.addInfo(fmt.Sprintf("%s → %s", id, value))
}

func (m *Model) handleSettingsOverlayKey(key string) bool {
	list, ok := m.overlayComp.(*SettingsList)
	if !ok || list == nil {
		return false
	}
	switch key {
	case "up", "shift+tab", "k":
		list.HandleInput("\x1b[A")
	case "down", "tab", "j":
		list.HandleInput("\x1b[B")
	case "enter", " ":
		list.HandleInput("\r")
	case "esc":
		m.closeOverlay()
		return true
	default:
		return false
	}
	m.syncOverlayFromComp()
	return true
}

// legacy string-рендерер, оставлен для тестов
func renderSettingsOverlay(items []settingItem, cursor int, s config.Settings) string {
	var b strings.Builder
	b.WriteString("settings (↑/↓ select, enter/space cycle, esc close)\n")
	for i, it := range items {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		val := settingValue(s, it.key)
		fmt.Fprintf(&b, "%s%s = %s\n", prefix, it.key, val)
		fmt.Fprintf(&b, "    %s\n", it.desc)
	}
	return b.String()
}

type settingItem struct {
	key  string
	desc string
}

var settingsItems = []settingItem{
	{key: "defaultModel", desc: "Default model name"},
	{key: "externalEditor", desc: "Ctrl+G editor command"},
	{key: "markdownPager", desc: "External markdown preview pager"},
	{key: "theme", desc: "Theme name"},
	{key: "defaultThinkingLevel", desc: "off|low|medium|high|xhigh|max"},
	{key: "compaction.enabled", desc: "true|false auto-compact"},
	{key: "steeringMode", desc: "one-at-a-time|all"},
	{key: "followUpMode", desc: "one-at-a-time|all"},
}
