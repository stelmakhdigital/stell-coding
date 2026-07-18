package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	DefaultHTTPIdleTimeoutMs         = 300000
	DefaultWebsocketConnectTimeoutMs = 15000
)

type Settings struct {
	DefaultProvider      string                `json:"defaultProvider,omitempty"`
	DefaultModel         string                `json:"defaultModel,omitempty"`
	DefaultThinkingLevel string                `json:"defaultThinkingLevel,omitempty"`
	EnabledModels        []string              `json:"enabledModels,omitempty"`
	Compaction           CompactionSettings    `json:"compaction,omitempty"`
	BranchSummary        BranchSummarySettings `json:"branchSummary,omitempty"`
	Retry                RetrySettings         `json:"retry,omitempty"`
	SteeringMode         string                `json:"steeringMode,omitempty"`
	FollowUpMode         string                `json:"followUpMode,omitempty"`
	SessionDir           string                `json:"sessionDir,omitempty"`
	Theme                string                `json:"theme,omitempty"`
	ExternalEditor       string                `json:"externalEditor,omitempty"`
	MarkdownPager        string                `json:"markdownPager,omitempty"`
	DefaultProjectTrust  string                `json:"defaultProjectTrust,omitempty"`
	HideThinkingBlock    *bool                 `json:"hideThinkingBlock,omitempty"`
	ShowCacheMissNotices *bool                 `json:"showCacheMissNotices,omitempty"`
	ShowImages           *bool                 `json:"showImages,omitempty"`
	ShowHardwareCursor   *bool                 `json:"showHardwareCursor,omitempty"`
	DiffScroll           *bool                 `json:"diffScroll,omitempty"`
	AutoResizeImages     *bool                 `json:"autoResizeImages,omitempty"`
	ImageWidthCells      int                   `json:"imageWidthCells,omitempty"`
	EditorPaddingX       int                   `json:"editorPaddingX,omitempty"`
	DoubleEscapeAction   string                `json:"doubleEscapeAction,omitempty"` // fork|tree|none
	ClearOnShrink        *bool                 `json:"clearOnShrink,omitempty"`
	EnableInstallTelemetry *bool               `json:"enableInstallTelemetry,omitempty"`
	EnableAnalytics      *bool                 `json:"enableAnalytics,omitempty"`
	TrackingID           string                `json:"trackingId,omitempty"`
	LastChangelogVersion string                `json:"lastChangelogVersion,omitempty"`
	CollapseChangelog    *bool                 `json:"collapseChangelog,omitempty"`
	QuietStartup         *bool                 `json:"quietStartup,omitempty"`
	CodexTransport              string `json:"transport,omitempty"` // auto|sse|websocket (Codex)
	HTTPProxy                   string `json:"httpProxy,omitempty"`
	HTTPIdleTimeoutMs           int    `json:"httpIdleTimeoutMs,omitempty"`
	WebsocketConnectTimeoutMs   int    `json:"websocketConnectTimeoutMs,omitempty"`
	TreeFilterMode              string `json:"treeFilterMode,omitempty"`
	OutputPad                   int    `json:"outputPad,omitempty"`
	AutocompleteMaxVisible      int    `json:"autocompleteMaxVisible,omitempty"`
	BlockImages                 *bool  `json:"blockImages,omitempty"`
	ShowTerminalProgress          *bool  `json:"showTerminalProgress,omitempty"`
	ThinkingBudgets             map[string]int        `json:"thinkingBudgets,omitempty"`
	Packages             []any                 `json:"packages,omitempty"`
	Extensions           []string              `json:"extensions,omitempty"`
	Skills               []string              `json:"skills,omitempty"`
	Prompts              []string              `json:"prompts,omitempty"`
	Themes               []string              `json:"themes,omitempty"`
	EnableSkillCommands  *bool                 `json:"enableSkillCommands,omitempty"`
	Security             SecuritySettings      `json:"security,omitempty"`
	Agent                AgentSettings         `json:"agent,omitempty"`
}

type AgentSettings struct {
	// MaxToolIterations опционально ограничивает tool loop за ход как safety
	// guardrail; 0 или unset — без лимита.
	MaxToolIterations int `json:"maxToolIterations,omitempty"`
	// ToolExecution — "parallel" (default) или "sequential" (настройка toolExecution).
	ToolExecution string `json:"toolExecution,omitempty"`
}

// ToolExecutionMode возвращает режим выполнения инструментов агента.
func (s Settings) ToolExecutionMode() string {
	m := s.Agent.ToolExecution
	if m == "" {
		return "parallel"
	}
	return m
}

type BranchSummarySettings struct {
	Enabled       *bool `json:"enabled,omitempty"`
	ReserveTokens int   `json:"reserveTokens,omitempty"`
	SkipPrompt    *bool `json:"skipPrompt,omitempty"`
}

type CompactionSettings struct {
	Enabled          *bool `json:"enabled,omitempty"`
	ReserveTokens    int   `json:"reserveTokens,omitempty"`
	KeepRecentTokens int   `json:"keepRecentTokens,omitempty"`
}

type RetryProviderSettings struct {
	TimeoutMs       int `json:"timeoutMs,omitempty"`
	MaxRetries      int `json:"maxRetries,omitempty"`
	MaxRetryDelayMs int `json:"maxRetryDelayMs,omitempty"`
}

type RetrySettings struct {
	Enabled     *bool `json:"enabled,omitempty"`
	MaxRetries  int   `json:"maxRetries,omitempty"`
	BaseDelayMs int   `json:"baseDelayMs,omitempty"`
	Provider    RetryProviderSettings `json:"provider,omitempty"`
}

func DefaultSettings() Settings {
	enabled := true
	skillCmd := true
	diffScroll := true
	return Settings{
		Compaction: CompactionSettings{
			Enabled:          &enabled,
			ReserveTokens:    16384,
			KeepRecentTokens: 20000,
		},
		BranchSummary: BranchSummarySettings{
			Enabled:       &enabled,
			ReserveTokens: 16384,
		},
		Retry: RetrySettings{
			Enabled:     &enabled,
			MaxRetries:  3,
			BaseDelayMs: 2000,
			Provider: RetryProviderSettings{
				MaxRetryDelayMs: 60000,
			},
		},
		DiffScroll:                  &diffScroll,
		TreeFilterMode:              "default",
		OutputPad:                   1,
		AutocompleteMaxVisible:      5,
		HTTPIdleTimeoutMs:           DefaultHTTPIdleTimeoutMs,
		WebsocketConnectTimeoutMs:   DefaultWebsocketConnectTimeoutMs,
		SteeringMode:                "one-at-a-time",
		FollowUpMode:                "one-at-a-time",
		EnableSkillCommands:         &skillCmd,
		DefaultProjectTrust:         "ask",
	}
}

// MaxToolIterations возвращает опциональный лимит tool loop; <= 0 — без лимита.
func (s Settings) MaxToolIterations() int {
	return s.Agent.MaxToolIterations
}

func (s Settings) Validate() error {
	if s.SteeringMode != "" && s.SteeringMode != "all" && s.SteeringMode != "one-at-a-time" {
		return fmt.Errorf("steeringMode must be all or one-at-a-time")
	}
	if s.FollowUpMode != "" && s.FollowUpMode != "all" && s.FollowUpMode != "one-at-a-time" {
		return fmt.Errorf("followUpMode must be all or one-at-a-time")
	}
	if s.DefaultProjectTrust != "" && s.DefaultProjectTrust != "ask" && s.DefaultProjectTrust != "always" && s.DefaultProjectTrust != "never" {
		return fmt.Errorf("defaultProjectTrust must be ask, always, or never")
	}
	return nil
}

func mergeSettings(base, over Settings) Settings {
	out := base
	if over.DefaultProvider != "" {
		out.DefaultProvider = over.DefaultProvider
	}
	if over.DefaultModel != "" {
		out.DefaultModel = over.DefaultModel
	}
	if over.DefaultThinkingLevel != "" {
		out.DefaultThinkingLevel = over.DefaultThinkingLevel
	}
	if len(over.EnabledModels) > 0 {
		out.EnabledModels = over.EnabledModels
	}
	if over.SteeringMode != "" {
		out.SteeringMode = over.SteeringMode
	}
	if over.FollowUpMode != "" {
		out.FollowUpMode = over.FollowUpMode
	}
	if over.SessionDir != "" {
		out.SessionDir = over.SessionDir
	}
	if over.Theme != "" {
		out.Theme = over.Theme
	}
	if over.ExternalEditor != "" {
		out.ExternalEditor = over.ExternalEditor
	}
	if over.MarkdownPager != "" {
		out.MarkdownPager = over.MarkdownPager
	}
	if over.DefaultProjectTrust != "" {
		out.DefaultProjectTrust = over.DefaultProjectTrust
	}
	if over.HideThinkingBlock != nil {
		out.HideThinkingBlock = over.HideThinkingBlock
	}
	if over.ShowCacheMissNotices != nil {
		out.ShowCacheMissNotices = over.ShowCacheMissNotices
	}
	if over.ShowImages != nil {
		out.ShowImages = over.ShowImages
	}
	if over.ShowHardwareCursor != nil {
		out.ShowHardwareCursor = over.ShowHardwareCursor
	}
	if over.DiffScroll != nil {
		out.DiffScroll = over.DiffScroll
	}
	if over.ImageWidthCells > 0 {
		out.ImageWidthCells = over.ImageWidthCells
	}
	if over.EditorPaddingX > 0 {
		out.EditorPaddingX = over.EditorPaddingX
	}
	if over.DoubleEscapeAction != "" {
		out.DoubleEscapeAction = over.DoubleEscapeAction
	}
	if over.ClearOnShrink != nil {
		out.ClearOnShrink = over.ClearOnShrink
	}
	if over.EnableInstallTelemetry != nil {
		out.EnableInstallTelemetry = over.EnableInstallTelemetry
	}
	if over.EnableAnalytics != nil {
		out.EnableAnalytics = over.EnableAnalytics
	}
	if over.TrackingID != "" {
		out.TrackingID = over.TrackingID
	}
	if over.LastChangelogVersion != "" {
		out.LastChangelogVersion = over.LastChangelogVersion
	}
	if over.CollapseChangelog != nil {
		out.CollapseChangelog = over.CollapseChangelog
	}
	if over.QuietStartup != nil {
		out.QuietStartup = over.QuietStartup
	}
	if over.CodexTransport != "" {
		out.CodexTransport = over.CodexTransport
	}
	if over.HTTPProxy != "" {
		out.HTTPProxy = over.HTTPProxy
	}
	if over.HTTPIdleTimeoutMs != 0 {
		out.HTTPIdleTimeoutMs = over.HTTPIdleTimeoutMs
	}
	if over.WebsocketConnectTimeoutMs != 0 {
		out.WebsocketConnectTimeoutMs = over.WebsocketConnectTimeoutMs
	}
	if over.TreeFilterMode != "" {
		out.TreeFilterMode = NormalizeTreeFilterMode(over.TreeFilterMode)
	}
	if over.OutputPad == 0 || over.OutputPad == 1 {
		out.OutputPad = over.OutputPad
	}
	if over.AutocompleteMaxVisible >= 3 && over.AutocompleteMaxVisible <= 20 {
		out.AutocompleteMaxVisible = over.AutocompleteMaxVisible
	}
	if over.BlockImages != nil {
		out.BlockImages = over.BlockImages
	}
	if over.ShowTerminalProgress != nil {
		out.ShowTerminalProgress = over.ShowTerminalProgress
	}
	if over.AutoResizeImages != nil {
		out.AutoResizeImages = over.AutoResizeImages
	}
	if len(over.ThinkingBudgets) > 0 {
		if out.ThinkingBudgets == nil {
			out.ThinkingBudgets = map[string]int{}
		}
		for k, v := range over.ThinkingBudgets {
			out.ThinkingBudgets[k] = v
		}
	}
	if len(over.Packages) > 0 {
		out.Packages = over.Packages
	}
	if len(over.Extensions) > 0 {
		out.Extensions = over.Extensions
	}
	if len(over.Skills) > 0 {
		out.Skills = over.Skills
	}
	if len(over.Prompts) > 0 {
		out.Prompts = over.Prompts
	}
	if len(over.Themes) > 0 {
		out.Themes = over.Themes
	}
	if over.EnableSkillCommands != nil {
		out.EnableSkillCommands = over.EnableSkillCommands
	}
	out.Compaction = mergeCompaction(base.Compaction, over.Compaction)
	out.BranchSummary = mergeBranchSummary(base.BranchSummary, over.BranchSummary)
	out.Retry = mergeRetry(base.Retry, over.Retry)
	if over.Agent.MaxToolIterations > 0 {
		out.Agent.MaxToolIterations = over.Agent.MaxToolIterations
	}
	if len(over.Security.ExtensionGrants) > 0 {
		if out.Security.ExtensionGrants == nil {
			out.Security.ExtensionGrants = map[string]bool{}
		}
		for k, v := range over.Security.ExtensionGrants {
			out.Security.ExtensionGrants[k] = v
		}
	}
	return out
}

func mergeBranchSummary(base, over BranchSummarySettings) BranchSummarySettings {
	out := base
	if over.Enabled != nil {
		out.Enabled = over.Enabled
	}
	if over.ReserveTokens > 0 {
		out.ReserveTokens = over.ReserveTokens
	}
	if over.SkipPrompt != nil {
		out.SkipPrompt = over.SkipPrompt
	}
	return out
}

func mergeCompaction(base, over CompactionSettings) CompactionSettings {
	out := base
	if over.Enabled != nil {
		out.Enabled = over.Enabled
	}
	if over.ReserveTokens > 0 {
		out.ReserveTokens = over.ReserveTokens
	}
	if over.KeepRecentTokens > 0 {
		out.KeepRecentTokens = over.KeepRecentTokens
	}
	return out
}

func mergeRetry(base, over RetrySettings) RetrySettings {
	out := base
	if over.Enabled != nil {
		out.Enabled = over.Enabled
	}
	if over.MaxRetries > 0 {
		out.MaxRetries = over.MaxRetries
	}
	if over.BaseDelayMs > 0 {
		out.BaseDelayMs = over.BaseDelayMs
	}
	out.Provider = mergeRetryProvider(base.Provider, over.Provider)
	return out
}

func mergeRetryProvider(base, over RetryProviderSettings) RetryProviderSettings {
	out := base
	if over.TimeoutMs > 0 {
		out.TimeoutMs = over.TimeoutMs
	}
	if over.MaxRetries > 0 {
		out.MaxRetries = over.MaxRetries
	}
	if over.MaxRetryDelayMs > 0 {
		out.MaxRetryDelayMs = over.MaxRetryDelayMs
	}
	return out
}

func (s Settings) CompactionEnabled() bool {
	return s.Compaction.Enabled == nil || *s.Compaction.Enabled
}

func (s Settings) BranchSummaryEnabled() bool {
	return s.BranchSummary.Enabled == nil || *s.BranchSummary.Enabled
}

func (s Settings) HideThinking() bool {
	return s.HideThinkingBlock != nil && *s.HideThinkingBlock
}

// ImagesEnabled по умолчанию true (inline-изображения в терминале при поддержке).
func (s Settings) ImagesEnabled() bool {
	return s.ShowImages == nil || *s.ShowImages
}

// HardwareCursorEnabled по умолчанию false (block glyph); true использует CursorMarker.
func (s Settings) HardwareCursorEnabled() bool {
	return s.ShowHardwareCursor != nil && *s.ShowHardwareCursor
}

// DiffScrollEnabled включает стратегию viewport scroll DiffEngine (по умолчанию on).
func (s Settings) DiffScrollEnabled() bool {
	return s.DiffScroll == nil || *s.DiffScroll
}

// AutoResizeImagesEnabled по умолчанию true — масштабировать изображения под бюджет cell/width.
func (s Settings) AutoResizeImagesEnabled() bool {
	return s.AutoResizeImages == nil || *s.AutoResizeImages
}

// ClearOnShrinkEnabled по умолчанию false.
func (s Settings) ClearOnShrinkEnabled() bool {
	return s.ClearOnShrink != nil && *s.ClearOnShrink
}

// DoubleEscapeActionOrDefault возвращает fork|tree|none (по умолчанию tree).
func (s Settings) DoubleEscapeActionOrDefault() string {
	switch strings.ToLower(strings.TrimSpace(s.DoubleEscapeAction)) {
	case "fork", "tree", "none":
		return strings.ToLower(strings.TrimSpace(s.DoubleEscapeAction))
	case "clear", "interrupt": // устаревшие псевдонимы
		if strings.EqualFold(s.DoubleEscapeAction, "clear") {
			return "fork"
		}
		return "tree"
	default:
		return "tree"
	}
}

// InstallTelemetryEnabled по умолчанию true.
func (s Settings) InstallTelemetryEnabled() bool {
	return s.EnableInstallTelemetry == nil || *s.EnableInstallTelemetry
}

// AnalyticsEnabled по умолчанию false.
func (s Settings) AnalyticsEnabled() bool {
	return s.EnableAnalytics != nil && *s.EnableAnalytics
}

// QuietStartupEnabled по умолчанию false.
func (s Settings) QuietStartupEnabled() bool {
	return s.QuietStartup != nil && *s.QuietStartup
}

// CollapseChangelogEnabled по умолчанию false.
func (s Settings) CollapseChangelogEnabled() bool {
	return s.CollapseChangelog != nil && *s.CollapseChangelog
}

func (s Settings) ShowCacheMiss() bool {
	return s.ShowCacheMissNotices == nil || *s.ShowCacheMissNotices
}

// NormalizeTreeFilterMode отображает kebab-case значения конфига во внутренние camelCase id фильтров.
func NormalizeTreeFilterMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "no-tools", "notools":
		return "noTools"
	case "user-only", "useronly":
		return "userOnly"
	case "labeled-only", "labeledonly":
		return "labeledOnly"
	case "all":
		return "all"
	default:
		return "default"
	}
}

func (s Settings) TreeFilterModeOrDefault() string {
	if m := NormalizeTreeFilterMode(s.TreeFilterMode); m != "" {
		return m
	}
	return "default"
}

func (s Settings) OutputPadOrDefault() int {
	if s.OutputPad == 0 || s.OutputPad == 1 {
		return s.OutputPad
	}
	return 1
}

func (s Settings) AutocompleteMaxVisibleOrDefault() int {
	if s.AutocompleteMaxVisible >= 3 && s.AutocompleteMaxVisible <= 20 {
		return s.AutocompleteMaxVisible
	}
	return 5
}

func (s Settings) HTTPIdleTimeoutOrDefault() int {
	if s.HTTPIdleTimeoutMs > 0 {
		return s.HTTPIdleTimeoutMs
	}
	if s.HTTPIdleTimeoutMs == 0 {
		return 0
	}
	return DefaultHTTPIdleTimeoutMs
}

func (s Settings) WebsocketConnectTimeoutOrDefault() int {
	if s.WebsocketConnectTimeoutMs > 0 {
		return s.WebsocketConnectTimeoutMs
	}
	if s.WebsocketConnectTimeoutMs == 0 {
		return 0
	}
	return DefaultWebsocketConnectTimeoutMs
}

func (s Settings) BranchSummaryReserveTokensOrDefault() int {
	if s.BranchSummary.ReserveTokens > 0 {
		return s.BranchSummary.ReserveTokens
	}
	return 16384
}

func (s Settings) BranchSummarySkipPrompt() bool {
	return s.BranchSummary.SkipPrompt != nil && *s.BranchSummary.SkipPrompt
}

func (s Settings) BlockImagesEnabled() bool {
	return s.BlockImages != nil && *s.BlockImages
}

func (s Settings) ShowTerminalProgressEnabled() bool {
	return s.ShowTerminalProgress != nil && *s.ShowTerminalProgress
}

// ApplyHTTPProxy задаёт HTTP_PROXY/HTTPS_PROXY, когда настроен httpProxy.
func ApplyHTTPProxy(proxy string) {
	proxy = strings.TrimSpace(proxy)
	if proxy == "" {
		return
	}
	if os.Getenv("HTTP_PROXY") == "" {
		_ = os.Setenv("HTTP_PROXY", proxy)
	}
	if os.Getenv("HTTPS_PROXY") == "" {
		_ = os.Setenv("HTTPS_PROXY", proxy)
	}
}

// ExternalEditorCommand возвращает команду редактора Ctrl+G: settings.externalEditor,
// затем $VISUAL, затем $EDITOR, затем notepad (Windows) или nano.
func (s Settings) ExternalEditorCommand() string {
	if e := strings.TrimSpace(s.ExternalEditor); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "nano"
}

// MarkdownPagerCommand возвращает внешний pager для markdown preview:
// settings.markdownPager, затем $PAGER, затем "less -R".
func (s Settings) MarkdownPagerCommand() string {
	if p := strings.TrimSpace(s.MarkdownPager); p != "" {
		return p
	}
	if p := os.Getenv("PAGER"); p != "" {
		// Предпочитаем ANSI-capable less, когда $PAGER — просто "less".
		if p == "less" {
			return "less -R"
		}
		return p
	}
	return "less -R"
}

// SaveGlobalSettings пишет settings.json в глобальный каталог агента.
func SaveGlobalSettings(globalDir string, s Settings) error {
	path := filepath.Join(globalDir, "settings.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
