// Package tui — интерактивный terminal UI с дифференциальным рендерингом.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stell/coding-agent/internal/agent"
	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
	"stell/coding-agent/internal/extensions"
	"stell/coding-agent/internal/themes"
	"stell/coding-agent/internal/update"
)

type Options struct {
	Service            *agent.Service
	Config             *config.Config
	GrantCh            <-chan extensions.GrantRequest
	UICh               <-chan extensions.UIRequest
	WorkflowCh         <-chan map[string]any
	ExternalPromptCh   <-chan agent.ExternalPrompt
}

type cardKind string

const (
	cardUser      cardKind = "user"
	cardAssistant cardKind = "assistant"
	cardThinking  cardKind = "thinking"
	cardTool      cardKind = "tool"
	cardBash      cardKind = "bash"
	cardSkill     cardKind = "skill"
	cardInfo      cardKind = "info"
	cardError     cardKind = "error"
	cardWarning   cardKind = "warning"
)

type cardStatus int

const (
	cardStatusNone cardStatus = iota
	cardStatusPending
	cardStatusSuccess
	cardStatusError
)

type card struct {
	kind      cardKind
	body      string
	skillName string
	skillBody string
	userTail  string
	images    []ai.ImageContent

	// Метаданные карточки tool/bash (фон статуса + Elapsed/Took + per-tool render).
	toolName    string
	toolCallID  string // agent tool call id for progress/result matching
	toolPath    string // path or command summary
	toolContent string // raw output / file body (without header)
	status      cardStatus
	startedAt   time.Time
	endedAt     time.Time
	timeoutSec  int
	excludeBash bool
}

type Model struct {
	svc    *agent.Service
	cfg    *config.Config
	colors palette
	keys   Keybindings
	activeTheme themes.Theme
	themeCtrl   *themes.Controller

	lines    []card
	viewport simpleViewport
	composer composerState
	loader   Loader

	width  int
	height int
	cellW  int // ширина ячейки терминала в пикселях (CSI 16 t)
	cellH  int

	busy          bool
	streaming     bool
	streamBuf     *strings.Builder
	showStartup   bool
	startupNotices  []card
	overlay       string
	overlayComp   Component
	overlayStack  []overlayFrame
	overlayMode   overlayMode
	overlayCursor int
	overlayAnchor overlayAnchor
	overlayMaxHeightPct int
	treeItems     []treeItem
	pickerFiles   []string
	pickerQuery   string
	modelNames    []string
	overlayList   string
	grantReq      *extensions.GrantRequest
	slashMenu     *slashMenuState
	bashMode      bool
	bashStreamIdx int
	bashCh        chan Msg
	pendingCmd    Cmd
	attachments   []composerAttachment
	attachmentFocus int
	cardIndex     int
	focus         focusArea
	treeFilter    string
	treeSearch    string
	treeFolded    map[string]bool
	treeShowTs    bool
	autocomplete  *acState
	sessionItems  []sessionItem
	uiOverlay     *uiOverlayState
	workflow      *workflowPanel
	fileIndex     []string
	thinkingCollapsed bool
	scopedEnabled map[string]bool
	sessionSortDesc   bool
	sessionNamedOnly  bool
	pendingDeleteSession string
	streamThinkBuf *strings.Builder
	errLine          string
	transientNotice  string
	statusLine       string // полоса Retry / Compacting / BranchSummary
	retryUntil       time.Time
	retryInfo        *agent.AutoRetryInfo
	overlayKeys      *KeyMap
	editorKeys       *KeybindingsManager
	scopedProvider string
	renameInput   string
	thinkStreamIdx int
	previewLines  []string
	previewScroll int
	extTitle          string
	extHeader         string
	extWidget         string
	extAbove          string
	extBelow          string
	extFooter         string
	extWorking        string
	extWorkingFrames  []string
	extWorkingEveryMs int
	extReplaceEditor  string

	lastCtrlCTime time.Time
	lastEscTime   time.Time
	prevHeight    int
	gitBranch     string
	forceFullRedraw bool

	events      <-chan agent.Event
	grantCh     <-chan extensions.GrantRequest
	uiCh        <-chan extensions.UIRequest
	workflowCh  <-chan map[string]any
	extPromptCh <-chan agent.ExternalPrompt
	ctx     context.Context
}

type agentEventMsg struct{ ev agent.Event }
type eventsClosedMsg struct{}
type errMsg struct{ err error }
type grantRequestMsg struct{ req extensions.GrantRequest }
type uiRequestMsg struct{ req extensions.UIRequest }
type externalPromptMsg struct{ prompt agent.ExternalPrompt }

func (m *Model) waitExternalPrompt() Cmd {
	ch := m.extPromptCh
	return func() Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return externalPromptMsg{prompt: p}
	}
}
type workflowEventMsg struct{ data map[string]any }

func (m *Model) waitUI() Cmd {
	ch := m.uiCh
	return func() Msg {
		req, ok := <-ch
		if !ok {
			return nil
		}
		return uiRequestMsg{req: req}
	}
}

func NewModel(ctx context.Context, opts Options) Model {
	th := loadTheme(opts.Config)
	pal := paletteFromTheme(th)
	m := Model{
		svc:         opts.Service,
		cfg:         opts.Config,
		ctx:         ctx,
		colors:      pal,
		activeTheme: th,
		keys:        LoadKeybindingsWithExtensions(opts.Config.GlobalDir, opts.Service.Extensions),
		composer:    newComposer(),
		loader:      Loader{Label: "thinking"},
		viewport:    newViewport(80, 20),
		grantCh:     opts.GrantCh,
		uiCh:        opts.UICh,
		workflowCh:  opts.WorkflowCh,
		extPromptCh:    opts.ExternalPromptCh,
		showStartup:    true,
		streamBuf:      &strings.Builder{},
		streamThinkBuf: &strings.Builder{},
		thinkStreamIdx: -1,
		bashStreamIdx:  -1,
		overlayKeys:    defaultOverlayKeyMap(),
		editorKeys:     NewKeybindingsManager(DefaultTUIKeybindings(), nil),
	}
	if opts.Config != nil {
		m.editorKeys = NewKeybindingsManager(DefaultTUIKeybindings(), LoadUserKeyOverrides(opts.Config.GlobalDir))
		m.treeFilter = opts.Config.Settings.TreeFilterModeOrDefault()
		m.themeCtrl = themes.NewController(themes.ResolveOpts{
			GlobalDir:  opts.Config.GlobalDir,
			ProjectDir: opts.Config.ProjectDir,
			Workspace:  opts.Config.Workspace,
		}, nil)
		applied := m.themeCtrl.ApplyFromSettings(opts.Config.Settings.Theme)
		m.activeTheme = applied
		m.colors = paletteFromTheme(applied)
	}
	m.rebuildOverlayKeys()
	m.hydrateSession()
	return m
}

func loadTheme(cfg *config.Config) themes.Theme {
	if cfg == nil {
		return themes.DefaultTheme()
	}
	opts := themes.ResolveOpts{
		GlobalDir:  cfg.GlobalDir,
		ProjectDir: cfg.ProjectDir,
		Workspace:  cfg.Workspace,
	}
	name := strings.TrimSpace(cfg.Settings.Theme)
	if name != "" {
		if resolved := themes.ResolveThemeSetting(name, themes.DetectDefaultName()); resolved != "" {
			name = resolved
		}
		if t := themes.FindByName(opts, name); t != nil {
			return *t
		}
	}
	list, err := themes.Resolve(opts)
	if err != nil || len(list) == 0 {
		return themes.DefaultTheme()
	}
	return list[0]
}

func paletteFromTheme(t themes.Theme) palette {
	c := t.Colors
	get := func(k, fallback string) string {
		if v, ok := c[k]; ok && v != "" {
			return v
		}
		return fallback
	}
	def := defaultPalette()
	fg := get("text", def.Foreground)
	if fg == "" {
		fg = def.Foreground
	}
	return palette{
		Accent:     get("accent", def.Accent),
		Foreground: fg,
		Muted:      get("muted", def.Muted),
		UserBlock:  get("userMessageBg", get("userBlock", def.UserBlock)),
		Assistant:  get("toolTitle", get("assistant", def.Assistant)),
		Tool:       get("toolTitle", get("tool", def.Tool)),
		Error:      get("error", def.Error),
		Border:     get("border", def.Border),
		Tokens:     c,
	}
}

func Run(ctx context.Context, opts Options) error {
	return runInteractive(ctx, opts)
}

func (m *Model) Init() Cmd {
	// tickMsg генерируется ticker'ом runInteractive (не самоперезапускающаяся cmd).
	cmds := []Cmd{m.indexWorkspaceFiles()}
	if m.grantCh != nil {
		cmds = append(cmds, m.waitGrant())
	}
	if m.uiCh != nil {
		cmds = append(cmds, m.waitUI())
	}
	if m.workflowCh != nil {
		cmds = append(cmds, m.waitWorkflow())
	}
	if m.extPromptCh != nil {
		cmds = append(cmds, m.waitExternalPrompt())
	}
	if !update.Offline() {
		cmds = append(cmds, m.checkLatestVersion(), m.checkPackageUpdates())
	}
	return Batch(cmds...)
}

func (m *Model) waitWorkflow() Cmd {
	ch := m.workflowCh
	return func() Msg {
		data, ok := <-ch
		if !ok {
			return nil
		}
		return workflowEventMsg{data: data}
	}
}

func (m *Model) waitGrant() Cmd {
	ch := m.grantCh
	return func() Msg {
		req, ok := <-ch
		if !ok {
			return nil
		}
		return grantRequestMsg{req: req}
	}
}

func (m Model) Update(msg Msg) (Model, Cmd) {
	var cmds []Cmd

	switch msg := msg.(type) {
	case WindowSizeMsg:
		if m.cfg != nil && m.cfg.Settings.ClearOnShrinkEnabled() && m.prevHeight > 0 && msg.Height < m.prevHeight {
			m.forceFullRedraw = true
			m.viewport.GotoBottom()
		}
		m.width, m.height = msg.Width, msg.Height
		m.prevHeight = msg.Height
		m.resizeViewport()
		m.syncViewport()
		return m, nil

	case grantRequestMsg:
		m.openGrantOverlay(msg.req)
		cmds = append(cmds, m.waitGrant())
		return m, Batch(cmds...)

	case uiRequestMsg:
		m.openUIOverlay(msg.req)
		cmds = append(cmds, m.waitUI())
		return m, Batch(cmds...)

	case workflowEventMsg:
		m.handleWorkflowEvent(msg.data)
		cmds = append(cmds, m.waitWorkflow())
		return m, Batch(cmds...)

	case externalPromptMsg:
		followUp := msg.prompt.DeliverAs == "followUp"
		if msg.prompt.DeliverAs == "steer" || (msg.prompt.DeliverAs == "" && m.busy) {
			followUp = false
		}
		cmds = append(cmds, m.submitWithText(msg.prompt.Message, followUp))
		cmds = append(cmds, m.waitExternalPrompt())
		return m, Batch(cmds...)

	case fileIndexMsg:
		if msg.err == nil {
			m.fileIndex = msg.files
		}
		return m, nil

	case themeReloadMsg:
		m.activeTheme = msg.theme
		m.colors = paletteFromTheme(msg.theme)
		m.forceFullRedraw = true
		m.syncViewport()
		return m, nil

	case versionCheckMsg:
		if msg.release != nil {
			m.showNewVersionNotification(*msg.release)
		}
		return m, nil

	case packageUpdatesMsg:
		if len(msg.names) > 0 {
			m.showPackageUpdateNotification(msg.names)
		}
		return m, nil

	case KeyMsg:
		keyStr := msg.String()
		if keyStr != "" && !isPasteKey(m.keys, keyStr) {
			m.errLine = ""
		}
		if msg.Paste && m.overlay == "" {
			if m.tryPasteClipboardImage() {
				m.syncViewport()
				return m, nil
			}
		}
		if m.overlayActive() {
			if m.handleOverlayKey(keyStr) {
				m.syncViewport()
				if m.pendingCmd != nil {
					cmd := m.pendingCmd
					m.pendingCmd = nil
					return m, cmd
				}
				return m, nil
			}
		}

		if m.overlayMode == overlayInlineAt && len(m.pickerFiles) > 0 {
			switch keyStr {
			case "up", "shift+tab":
				if m.overlayCursor > 0 {
					m.overlayCursor--
				}
				return m, nil
			case "down", "tab":
				if m.overlayCursor < len(m.pickerFiles)-1 {
					m.overlayCursor++
				}
				return m, nil
			case "enter":
				m.insertInlinePicker(m.pickerFiles[m.overlayCursor])
				m.syncViewport()
				return m, nil
			case "esc":
				m.closeOverlay()
				return m, nil
			}
		}

		if m.handleAttachmentKeys(keyStr) {
			return m, nil
		}

		if keyStr == "shift+enter" && m.overlay == "" {
			if m.composer.ed != nil {
				m.composer.ed.HandleInput("\n")
			}
			m.syncViewport()
			return m, nil
		}

		if action, ok := m.keys.ActionForKey(keyStr); ok {
			switch action {
			case actionClear:
				now := time.Now()
				if !m.lastCtrlCTime.IsZero() && now.Sub(m.lastCtrlCTime) < 500*time.Millisecond {
					return m, QuitCmd()
				}
				m.lastCtrlCTime = now
				m.composer.SetValue("")
				m.attachments = nil
				m.dismissPopups()
				m.errLine = ""
				m.syncViewport()
				return m, nil
			case actionFollowUp:
				return m, m.submit(true)
			case actionSubmit:
				if !msg.Alt {
					if m.slashMenu != nil {
						word := wordAtEnd(m.composer.Value())
						sel := m.slashMenu.items[m.slashMenu.index].name
						if strings.EqualFold(word, sel) {
							return m, m.submit(false)
						}
						m.applySlashSelection()
						m.syncViewport()
						return m, nil
					}
					return m, m.submit(false)
				}
			case actionScrollUp:
				m.viewport.LineUp(1)
				return m, nil
			case actionScrollDown:
				m.viewport.LineDown(1)
				return m, nil
			case actionModelCycle:
				m.cycleModel()
				m.syncViewport()
				return m, nil
			case actionModelSelect:
				m.openModelOverlay()
				m.syncViewport()
				return m, nil
			case actionTreeOpen:
				m.svc.EmitBeforeTree(m.ctx)
				m.openTreeOverlay()
				m.syncViewport()
				return m, nil
			case actionSettingsOpen:
				m.openSettingsOverlay()
				m.syncViewport()
				return m, nil
			case actionThinkingCycle:
				lv := m.svc.CycleThinkingLevel()
				m.addInfo("thinking → " + lv)
				m.syncViewport()
				return m, nil
			case actionSessionResume:
				m.openSessionOverlay()
				m.syncViewport()
				return m, nil
			case actionSessionNew:
				cmd := m.handleSlash("/new")
				if cmd == nil {
					m.syncViewport()
				}
				return m, cmd
			case actionMessageCopy:
				m.copyLastAssistant()
				m.syncViewport()
				return m, nil
			case actionMessageDequeue:
				if text, ok := m.svc.PopQueuedMessage(); ok {
					m.composer.SetValue(text)
					m.addInfo("restored queued message")
				} else {
					m.addInfo("no queued messages")
				}
				return m, nil
			case actionPasteClipboard:
				m.pasteFromClipboard()
				return m, nil
			case actionThinkingToggle:
				m.toggleThinkingCollapsed()
				m.syncViewport()
				return m, nil
			case actionExternalEditor:
				m.openExternalEditor()
				return m, nil
			case actionMarkdownPreview:
				m.openMarkdownPreview()
				m.syncViewport()
				return m, nil
			case actionScopedModels:
				m.openScopedModelsOverlay()
				m.syncViewport()
				return m, nil
			case actionModelCycleBack:
				m.cycleModelBackward()
				m.syncViewport()
				return m, nil
			case actionInterrupt:
				if m.svc.IsBashRunning() {
					m.svc.AbortBash()
					m.addInfo("aborting bash…")
					m.syncViewport()
					return m, nil
				}
				if m.overlayActive() {
					m.closeOverlay()
					m.syncViewport()
					return m, nil
				}
				if m.extReplaceEditor != "" {
					m.extReplaceEditor = ""
					m.syncViewport()
					return m, nil
				}
				if m.popupsActive() {
					m.dismissPopups()
					m.syncViewport()
					return m, nil
				}
				if m.busy {
					restored := m.svc.DrainQueues()
					m.svc.Abort()
					if len(restored) > 0 {
						m.composer.SetValue(strings.Join(restored, "\n\n"))
						m.addInfo("aborted · restored queued messages")
					} else {
						m.addInfo("aborting…")
					}
					m.syncViewport()
					return m, nil
				}
				if m.cfg == nil {
					return m, nil
				}
				// Двойной Esc в простое (doubleEscapeAction: fork|tree|none).
				now := time.Now()
				if !m.lastEscTime.IsZero() && now.Sub(m.lastEscTime) < 500*time.Millisecond {
					m.lastEscTime = time.Time{}
					switch m.cfg.Settings.DoubleEscapeActionOrDefault() {
					case "fork":
						m.openTreeOverlay()
						m.addInfo("tree (fork): select entry and enter")
						m.syncViewport()
					case "none":
						// игнорируем второй Esc
					default: // дерево
						m.openTreeOverlay()
						m.syncViewport()
					}
					return m, nil
				}
				m.lastEscTime = now
				return m, nil
			case actionDeleteForward:
				if m.composer.ed != nil {
					m.composer.ed.DeleteForward()
				}
				m.syncViewport()
				return m, nil
			case actionSuspend:
				return m, suspendCmd()
			case actionEditorYank:
				if m.composer.ed != nil {
					m.composer.ed.Yank()
				}
				m.syncViewport()
				return m, nil
			case actionEditorPageUp:
				if m.composer.ed != nil {
					m.composer.ed.PageUp()
				}
				m.syncViewport()
				return m, nil
			case actionEditorPageDown:
				if m.composer.ed != nil {
					m.composer.ed.PageDown()
				}
				m.syncViewport()
				return m, nil
			case actionTreeFold:
				if m.overlayMode == overlayTree {
					m.treeFoldOrUp()
					m.syncViewport()
				}
				return m, nil
			case actionTreeUnfold:
				if m.overlayMode == overlayTree {
					m.treeUnfoldOrDown()
					m.syncViewport()
				}
				return m, nil
			case actionSessionFork:
				if m.overlayMode == overlaySession {
					m.forkSessionFromOverlay()
				} else {
					m.openTreeOverlay()
				}
				m.syncViewport()
				return m, nil
			case actionTreeFilterCycle:
				if m.overlayMode == overlayTree {
					m.cycleTreeFilter()
				}
				return m, nil
			case actionTreeFilterCycleBack:
				if m.overlayMode == overlayTree {
					m.cycleTreeFilterBack()
				}
				return m, nil
			case actionTreeEditLabel:
				if m.overlayMode == overlayTree {
					m.editTreeLabel()
				}
				return m, nil
			default:
				if IsExtAction(action) && m.svc.Extensions != nil {
					name := ExtActionName(action)
					if err := m.svc.Extensions.InvokeShortcut(m.ctx, name); err != nil {
						m.addError(err.Error())
					} else {
						m.addInfo("shortcut: " + name)
					}
					return m, nil
				}
			}
		}

		if m.autocomplete != nil && (keyStr == "up" || keyStr == "down") {
			if keyStr == "up" && m.autocomplete.index > 0 {
				m.autocomplete.index--
			}
			if keyStr == "down" && m.autocomplete.index < len(m.autocomplete.items)-1 {
				m.autocomplete.index++
			}
			if m.slashMenu != nil {
				m.slashMenu.index = m.autocomplete.index
			}
			if m.overlayMode == overlayInlineAt {
				m.overlayCursor = m.autocomplete.index
			}
			m.syncViewport()
			return m, nil
		}

		if m.slashMenu != nil && (keyStr == "up" || keyStr == "down") {
			if keyStr == "up" && m.slashMenu.index > 0 {
				m.slashMenu.index--
			}
			if keyStr == "down" && m.slashMenu.index < len(m.slashMenu.items)-1 {
				m.slashMenu.index++
			}
			m.syncViewport()
			return m, nil
		}

		if (m.autocomplete != nil || m.slashMenu != nil) && keyStr == "tab" {
			if m.autocomplete != nil {
				m.applyAutocomplete()
			} else {
				m.applySlashSelection()
			}
			m.syncViewport()
			return m, nil
		}

		if m.overlay == "" && m.keys.Matches(actionCardFocus, keyStr) && m.slashMenu == nil && m.autocomplete == nil {
			if m.focus == focusInput {
				m.focus = focusCards
				m.scrollToCard()
			} else {
				m.focus = focusInput
			}
			return m, nil
		}

		if m.focus == focusCards && m.overlay == "" {
			if m.keys.Matches(actionCardUp, keyStr) {
				m.scrollCardUp()
				return m, nil
			}
			if m.keys.Matches(actionCardDown, keyStr) {
				m.scrollCardDown()
				return m, nil
			}
		}

		if !isBoundKey(m.keys, keyStr) || !isEditorKey(keyStr) {
			var cmd Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

		var cmd Cmd
		prev := m.composer.Value()
		if m.composer.keys == nil && m.editorKeys != nil {
			m.composer.keys = m.editorKeys
		}
		m.composer, cmd = m.composer.Update(msg)
		if m.composer.Value() != prev {
			m.attachmentFocus = -1
			m.updateBashMode()
		}
		cmds = append(cmds, cmd)
		m.updateAutocomplete()
		m.resizeViewport()
		return m, Batch(cmds...)

	case tickMsg:
		m.loader.Advance()
		if m.retryInfo != nil && !m.retryUntil.IsZero() {
			m.syncRetryStatusLine()
		}
		if m.refreshPendingToolTiming() {
			m.syncViewport()
		}
		return m, nil

	case bashProgressMsg:
		cmd := m.handleBashProgress(msg)
		cmds = append(cmds, cmd)
		return m, Batch(cmds...)

	case bashDoneMsg:
		cmd := m.handleBashDone(msg)
		cmds = append(cmds, cmd)
		return m, Batch(cmds...)

	case agentEventMsg:
		m.applyEvent(msg.ev)
		cmds = append(cmds, m.waitEvent())
		return m, Batch(cmds...)

	case eventsClosedMsg:
		m.busy = false
		m.streaming = false
		m.streamBuf.Reset()
		m.syncViewport()
		return m, nil

	case errMsg:
		m.errLine = msg.err.Error()
		m.busy = false
		return m, nil
	}

	if m.overlay == "" {
		var cmd Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, Batch(cmds...)
}

func (m *Model) submit(followUp bool) Cmd {
	return m.submitWithText(strings.TrimSpace(m.composer.Value()), followUp)
}

func (m *Model) submitWithText(text string, followUp bool) Cmd {
	if text == "" && len(m.attachments) == 0 {
		return nil
	}

	if text != "" {
		if mod, cancel, err := m.svc.EmitInputHook(m.ctx, text); err != nil {
			m.addError(err.Error())
			m.syncViewport()
			return nil
		} else if cancel {
			m.syncViewport()
			return nil
		} else if mod != "" {
			text = mod
		}
	}

	if text != "" && strings.HasPrefix(text, "/") && m.isManagedSlash(text) {
		cmd := m.handleSlash(text)
		m.clearSubmitComposer()
		m.syncViewport()
		return cmd
	}

	if cmd, exclude, ok := parseUserBashInput(text); ok {
		if m.svc.IsBashRunning() {
			m.addInfo("A bash command is already running. Press Esc to cancel it first.")
			m.syncViewport()
			return nil
		}
		return m.startUserBash(cmd, exclude)
	}

	if text != "" && strings.HasPrefix(text, "@") {
		rest := strings.TrimSpace(strings.TrimPrefix(text, "@"))
		if rest != "" && !strings.ContainsAny(rest, " \n\t") {
			m.openPickerOverlay(rest)
			m.syncViewport()
			return nil
		}
	}

	if m.busy {
		behavior := "steer"
		if followUp {
			behavior = "followUp"
		}
		if err := m.prepareAttachments(); err != nil {
			m.addError(err.Error())
			m.syncViewport()
			return nil
		}
		events := make(chan agent.Event, 64)
		if err := m.svc.Prompt(m.ctx, text, behavior, events); err != nil {
			m.svc.SetPendingAttachments(nil)
			m.svc.SetPendingImages(nil)
			m.addError(err.Error())
			m.syncViewport()
			return nil
		}
		m.clearSubmitComposer()
		m.attachments = nil
		m.attachmentFocus = -1
		m.addInfo("queued (" + behavior + ")")
		m.syncViewport()
		return nil
	}

	events := make(chan agent.Event, 64)
	if err := m.prepareAttachments(); err != nil {
		m.addError(err.Error())
		m.syncViewport()
		return nil
	}
	if err := m.svc.Prompt(m.ctx, text, "", events); err != nil {
		m.svc.SetPendingAttachments(nil)
		m.svc.SetPendingImages(nil)
		m.addError(err.Error())
		m.syncViewport()
		return nil
	}
	m.clearSubmitComposer()
	m.busy = true
	m.showStartup = false
	m.attachments = nil
	m.attachmentFocus = -1
	m.events = events
	m.syncViewport()
	return m.waitEvent()
}

func (m *Model) waitEvent() Cmd {
	ch := m.events
	return func() Msg {
		ev, ok := <-ch
		if !ok {
			return eventsClosedMsg{}
		}
		return agentEventMsg{ev: ev}
	}
}

func (m *Model) applyEvent(ev agent.Event) {
	switch ev.Type {
	case agent.EventAutoRetryStart:
		if ev.AutoRetry != nil {
			m.retryInfo = ev.AutoRetry
			if ev.AutoRetry.DelayMs > 0 {
				m.retryUntil = time.Now().Add(time.Duration(ev.AutoRetry.DelayMs) * time.Millisecond)
			}
			m.syncRetryStatusLine()
		} else {
			m.statusLine = "Retrying…"
		}
	case agent.EventAutoRetryEnd:
		m.retryUntil = time.Time{}
		m.retryInfo = nil
		if ev.AutoRetry != nil && !ev.AutoRetry.Success && ev.AutoRetry.FinalError != "" {
			m.statusLine = "Retry failed: " + ev.AutoRetry.FinalError
		} else {
			m.statusLine = ""
		}
	case agent.EventThinkingToken:
		if m.cfg != nil && m.cfg.Settings.HideThinking() {
			return
		}
		m.streaming = true
		m.streamThinkBuf.WriteString(ev.Thinking)
		m.updateThinkingCard()
	case agent.EventToken:
		m.streaming = true
		m.streamBuf.WriteString(ev.Token)
		m.updateStreamingCard()
	case agent.EventMessageStart:
		m.streaming = true
		m.streamBuf.Reset()
		m.updateStreamingCard()
	case agent.EventMessage:
		m.transientNotice = ""
		switch ev.Message.Role {
		case ai.RoleUser:
			c := cardFromUserContent(ev.Message.Content)
			if len(ev.Message.Images) > 0 {
				c.images = append([]ai.ImageContent(nil), ev.Message.Images...)
			}
			m.lines = append(m.lines, c)
		case ai.RoleAssistant:
			m.finalizeStreaming(ev.Message.Content)
		}
	case agent.EventToolCall:
		if ev.ToolCall != nil {
			m.flushStreaming()
			m.flushThinking()
			m.addToolCall(ev.ToolCall.ID, ev.ToolCall.Name, ev.ToolCall.Args)
		}
	case agent.EventToolProgress:
		if ev.ToolResult != nil {
			m.updateToolStreaming(ev.ToolResult.CallID, ev.ToolResult.Name, ev.ToolResult.Content)
		}
	case agent.EventToolResult:
		if ev.ToolResult != nil {
			m.finalizeToolResult(ev.ToolResult.CallID, ev.ToolResult.Name, ev.ToolResult.Content, ev.ToolResult.Error, ev.ToolResult.FullOutputPath)
		}
	case agent.EventLabel:
		if ev.Notice != "" {
			m.addInfo(ev.Notice)
		}
	case agent.EventError:
		if ev.Err != nil {
			m.addError(ev.Err.Error())
		}
	case agent.EventDone:
		m.transientNotice = ""
		m.statusLine = ""
		m.retryUntil = time.Time{}
		m.retryInfo = nil
		m.flushStreaming()
		m.flushThinking()
		switch ev.StopReason {
		case "cancelled":
			m.addInfo("cancelled")
		case "truncated":
			m.showTransientNotice("response truncated (output token limit)")
		case "incomplete":
			m.showTransientNotice("response may be incomplete (stream ended early)")
		}
	case agent.EventNotice:
		if ev.Notice != "" {
			lower := strings.ToLower(ev.Notice)
			if strings.Contains(lower, "compact") {
				hint := KeyDisplay(m.keys, actionInterrupt)
				if hint == "" {
					hint = "esc"
				}
				m.statusLine = fmt.Sprintf("Compacting… (%s to cancel)", hint)
			}
			if strings.Contains(lower, "branch") && strings.Contains(lower, "summary") {
				hint := KeyDisplay(m.keys, actionInterrupt)
				if hint == "" {
					hint = "esc"
				}
				m.statusLine = fmt.Sprintf("Branch summary (%s to cancel)", hint)
			}
			// Notice помечают границы попыток (retry, auto-continue).
			// Сбрасываем буфер thinking, чтобы следующая попытка начала новую
			// карточку, а не перерисовывала reasoning всех предыдущих попыток.
			if isTransientAgentNotice(ev.Notice) {
				m.showTransientNotice(ev.Notice)
			} else {
				m.flushThinking()
				m.addInfo(ev.Notice)
			}
		}
	}
	m.syncViewport()
}

func (m *Model) updateThinkingCard() {
	text := m.streamThinkBuf.String()
	if m.thinkingCollapsed {
		key := KeyDisplay(m.keys, actionThinkingToggle)
		if key == "" {
			key = "ctrl+t"
		}
		text = fmt.Sprintf("thinking… (%s to expand)", key)
	}
	if m.thinkStreamIdx >= 0 && m.thinkStreamIdx < len(m.lines) && m.lines[m.thinkStreamIdx].kind == cardThinking {
		m.lines[m.thinkStreamIdx].body = text
		return
	}
	m.thinkStreamIdx = len(m.lines)
	m.lines = append(m.lines, card{kind: cardThinking, body: text})
}

func (m *Model) flushThinking() {
	m.streamThinkBuf.Reset()
	m.thinkStreamIdx = -1
}

func (m *Model) updateStreamingCard() {
	text := m.streamBuf.String()
	if m.streaming && len(m.lines) > 0 && m.lines[len(m.lines)-1].kind == cardAssistant {
		m.lines[len(m.lines)-1].body = text
		return
	}
	m.lines = append(m.lines, card{kind: cardAssistant, body: text})
}

// findToolCard возвращает индекс tool-карточки по callID или последнюю
// pending-карточку с тем же именем tool (fallback, если callID пуст).
func (m *Model) findToolCard(callID, name string) int {
	if callID != "" {
		for i := range m.lines {
			c := &m.lines[i]
			if c.kind == cardTool && c.toolCallID == callID {
				return i
			}
		}
	}
	if name != "" {
		for i := len(m.lines) - 1; i >= 0; i-- {
			c := &m.lines[i]
			if c.kind == cardTool && c.status == cardStatusPending && normalizeToolName(c.toolName) == normalizeToolName(name) {
				return i
			}
		}
	}
	return -1
}

func (m *Model) updateToolStreaming(callID, name, content string) {
	content = stripRunningHeartbeat(content)
	if idx := m.findToolCard(callID, name); idx >= 0 {
		c := &m.lines[idx]
		c.toolContent = content
		c.body = formatToolResult(name, content, "", "")
		c.status = cardStatusPending
		if callID != "" && c.toolCallID == "" {
			c.toolCallID = callID
		}
		if c.startedAt.IsZero() {
			c.startedAt = time.Now()
		}
		return
	}
	m.addToolCall(callID, name, nil)
	m.updateToolStreaming(callID, name, content)
}

func (m *Model) finalizeToolResult(callID, name, content, errText, fullOutputPath string) {
	content = stripRunningHeartbeat(content)
	body := formatToolResult(name, content, errText, fullOutputPath)
	status := cardStatusSuccess
	if errText != "" {
		status = cardStatusError
	}
	now := time.Now()
	if idx := m.findToolCard(callID, name); idx >= 0 {
		c := &m.lines[idx]
		c.body = body
		c.toolContent = content
		if errText != "" {
			c.toolContent = "error: " + errText
			if fullOutputPath != "" {
				c.toolContent += "\nFull output: " + fullOutputPath
			}
		} else if fullOutputPath != "" && !strings.Contains(content, fullOutputPath) {
			c.toolContent = content + "\nFull output: " + fullOutputPath
		}
		c.status = status
		c.endedAt = now
		if c.startedAt.IsZero() {
			c.startedAt = now
		}
		if c.toolName == "" {
			c.toolName = name
		}
		if callID != "" && c.toolCallID == "" {
			c.toolCallID = callID
		}
		return
	}
	// Нет подходящей pending-карточки (orphan result): всё равно пишем, но Took будет ~0.
	m.lines = append(m.lines, card{
		kind:        cardTool,
		body:        body,
		toolName:    name,
		toolCallID:  callID,
		toolContent: content,
		status:      status,
		startedAt:   now,
		endedAt:     now,
	})
}

func (m *Model) finalizeStreaming(content string) {
	if content != "" {
		if m.streaming && len(m.lines) > 0 && m.lines[len(m.lines)-1].kind == cardAssistant {
			m.lines[len(m.lines)-1].body = content
		} else {
			m.lines = append(m.lines, card{kind: cardAssistant, body: content})
		}
	}
	m.streaming = false
	m.streamBuf.Reset()
}

func (m *Model) flushStreaming() {
	if m.streaming {
		m.streaming = false
		m.streamBuf.Reset()
	}
}

func (m *Model) addToolCall(callID, name string, args map[string]any) {
	if callID != "" {
		if idx := m.findToolCard(callID, ""); idx >= 0 {
			c := &m.lines[idx]
			path := toolArgString(args, "command", "path", "file_path", "query", "pattern", "description")
			content := toolArgString(args, "content")
			timeout := toolTimeoutSec(args)
			c.body = formatToolCall(name, args)
			c.toolName = name
			c.toolPath = path
			if content != "" {
				c.toolContent = content
			}
			c.status = cardStatusPending
			if c.startedAt.IsZero() {
				c.startedAt = time.Now()
			}
			if timeout > 0 {
				c.timeoutSec = timeout
			}
			return
		}
	}
	path := toolArgString(args, "command", "path", "file_path", "query", "pattern", "description")
	content := toolArgString(args, "content")
	timeout := toolTimeoutSec(args)
	now := time.Now()
	c := card{
		kind:        cardTool,
		body:        formatToolCall(name, args),
		toolName:    name,
		toolCallID:  callID,
		toolPath:    path,
		toolContent: content,
		status:      cardStatusPending,
		startedAt:   now,
		timeoutSec:  timeout,
	}
	m.lines = append(m.lines, c)
}

// refreshPendingToolTiming возвращает true, если pending tool/bash карточку нужно перерисовать (Elapsed).
func (m *Model) refreshPendingToolTiming() bool {
	for i := range m.lines {
		c := &m.lines[i]
		if (c.kind == cardTool || c.kind == cardBash) && c.status == cardStatusPending && !c.startedAt.IsZero() {
			return true
		}
	}
	return false
}

func (m *Model) addInfo(text string) {
	m.lines = append(m.lines, card{kind: cardInfo, body: text})
}

func isTransientAgentNotice(notice string) bool {
	return strings.Contains(notice, "retrying (") ||
		strings.Contains(notice, "auto-retry") ||
		strings.Contains(notice, "temporarily unavailable")
}

func (m *Model) showTransientNotice(text string) {
	m.transientNotice = text
	m.flushThinking()
}

func (m *Model) addError(text string) {
	m.lines = append(m.lines, card{kind: cardError, body: formatUserError(text)})
}

func (m Model) View() string {
	root := &appRoot{m: &m}
	out := joinLines(root.Render(m.width))
	if m.height > 0 {
		for viewLineCount(out) < m.height {
			out += "\n"
		}
		for viewLineCount(out) > m.height {
			if i := strings.LastIndex(out, "\n"); i >= 0 {
				out = out[:i]
			} else {
				break
			}
		}
	}
	return out
}

func (m *Model) syncViewport() {
	m.viewport.SetContent(m.renderCards())
	m.viewport.GotoBottom()
}

func (m *Model) renderCards() string {
	var b strings.Builder
	lay := m.chatLayout()
	if m.showStartup && m.overlay == "" {
		if block := m.startupBanner(); block != "" {
			b.WriteString(block)
			b.WriteString("\n\n")
		}
	}
	for i, c := range m.startupNotices {
		line := m.finishCardLayout(c, m.renderCardBody(-1-i, c, lay), lay)
		b.WriteString(line)
		b.WriteString("\n\n")
	}
	if len(m.lines) == 0 && m.overlay == "" {
		if b.Len() == 0 {
			if m.showStartup {
				return m.startupBanner()
			}
			return indentLines(m.colors.muted().Render("Type a message or /help"), lay.textMargin)
		}
		return strings.TrimRight(b.String(), "\n")
	}
	for i, c := range m.lines {
		cardLay := lay
		if m.focus == focusCards && i == m.cardIndex {
			// Оставляем место под focus-префикс "> ", почти не сдвигая геометрию блока.
			if cardLay.contentW > 22 {
				cardLay.contentW -= 2
				cardLay.textW = cardLay.contentW - 2*cardLay.innerPad
				if cardLay.textW < 10 {
					cardLay.textW = 10
				}
			}
		}
		line := m.finishCardLayout(c, m.renderCardBody(i, c, cardLay), cardLay)
		if m.focus == focusCards && i == m.cardIndex {
			line = "> " + line
		}
		b.WriteString(line)
		b.WriteString("\n\n")
	}
	return b.String()
}

func trimPath(p string, max int) string {
	if len(p) <= max {
		return p
	}
	return "…" + p[len(p)-max+1:]
}

func (m *Model) reloadTheme() {
	if m.themeCtrl != nil && m.cfg != nil {
		th := m.themeCtrl.ApplyFromSettings(m.cfg.Settings.Theme)
		m.activeTheme = th
		m.colors = paletteFromTheme(th)
	} else {
		th := loadTheme(m.cfg)
		m.activeTheme = th
		m.colors = paletteFromTheme(th)
	}
	m.forceFullRedraw = true
	m.syncViewport()
}

func (m *Model) markdownTheme() MarkdownTheme {
	md := m.activeTheme.MarkdownTheme()
	out := MarkdownTheme{
		Heading: md.Heading, Link: md.Link, LinkURL: md.LinkURL,
		Code: md.Code, CodeBlock: md.CodeBlock, CodeBlockBorder: md.CodeBlockBorder,
		Quote: md.Quote, QuoteBorder: md.QuoteBorder, HR: md.HR, ListBullet: md.ListBullet,
		Strike: md.Strike, Bold: md.Bold, Italic: md.Italic, Reset: md.Reset,
	}
	if out.Reset == "" {
		return DefaultMarkdownTheme()
	}
	return out
}

func (m *Model) syncRetryStatusLine() {
	if m.retryInfo == nil {
		return
	}
	remaining := time.Duration(0)
	if !m.retryUntil.IsZero() {
		remaining = time.Until(m.retryUntil)
		if remaining < 0 {
			remaining = 0
		}
	}
	hint := KeyDisplay(m.keys, actionInterrupt)
	if hint == "" {
		hint = "esc"
	}
	if remaining > 0 {
		m.statusLine = fmt.Sprintf("Retry %d/%d in %.1fs (%s to cancel)", m.retryInfo.Attempt, m.retryInfo.MaxAttempts, remaining.Seconds(), hint)
		return
	}
	m.statusLine = fmt.Sprintf("Retry %d/%d (%s to cancel)", m.retryInfo.Attempt, m.retryInfo.MaxAttempts, hint)
}
