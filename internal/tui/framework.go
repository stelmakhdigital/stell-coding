package tui

import tuilib "github.com/stelmakhdigital/stell-tui"

// Типы фреймворка из github.com/stelmakhdigital/stell-tui (DiffEngine, components, editor).
type (
	DiffStrategy         = tuilib.DiffStrategy
	Component            = tuilib.Component
	Focusable            = tuilib.Focusable
	InputHandler         = tuilib.InputHandler
	Container            = tuilib.Container
	DiffEngine           = tuilib.DiffEngine
	TUI                  = tuilib.TUI
	Text                 = tuilib.Text
	Box                  = tuilib.Box
	Loader               = tuilib.Loader
	SelectList           = tuilib.SelectList
	SettingsList         = tuilib.SettingsList
	SettingsItem         = tuilib.SettingsItem
	Markdown             = tuilib.Markdown
	MarkdownTheme        = tuilib.MarkdownTheme
	Editor               = tuilib.Editor
	KillRing             = tuilib.KillRing
	Image                = tuilib.Image
	Spacer               = tuilib.Spacer
	TruncatedText        = tuilib.TruncatedText
	KeyMap               = tuilib.KeyMap
	KeybindingsManager   = tuilib.KeybindingsManager
	Autocomplete         = tuilib.Autocomplete
	CompleteItem         = tuilib.CompleteItem
	TerminalCapabilities = tuilib.TerminalCapabilities
	CancellableLoader    = tuilib.CancellableLoader
	Input                = tuilib.Input
	ProcessTerminal      = tuilib.ProcessTerminal
	OverlayHandle        = tuilib.OverlayHandle
	OverlayMargin        = tuilib.OverlayMargin
	UnfocusOptions       = tuilib.UnfocusOptions
	InputListener        = tuilib.InputListener
	Terminal             = tuilib.Terminal
)

const (
	DiffFull   = tuilib.DiffFull
	DiffPatch  = tuilib.DiffPatch
	DiffScroll = tuilib.DiffScroll

	OverlayAnchorTop          = tuilib.OverlayAnchorTop
	OverlayAnchorCenter       = tuilib.OverlayAnchorCenter
	OverlayAnchorBottom       = tuilib.OverlayAnchorBottom
	OverlayAnchorTopLeft      = tuilib.OverlayAnchorTopLeft
	OverlayAnchorTopRight     = tuilib.OverlayAnchorTopRight
	OverlayAnchorBottomLeft   = tuilib.OverlayAnchorBottomLeft
	OverlayAnchorBottomRight  = tuilib.OverlayAnchorBottomRight
	OverlayAnchorTopCenter    = tuilib.OverlayAnchorTopCenter
	OverlayAnchorBottomCenter = tuilib.OverlayAnchorBottomCenter
	OverlayAnchorLeftCenter   = tuilib.OverlayAnchorLeftCenter
	OverlayAnchorRightCenter  = tuilib.OverlayAnchorRightCenter
)

var (
	New                    = tuilib.New
	NewWithTerminal        = tuilib.NewWithTerminal
	NewContainer           = tuilib.NewContainer
	NewDiffEngine          = tuilib.NewDiffEngine
	NewSelectList          = tuilib.NewSelectList
	NewSettingsList        = tuilib.NewSettingsList
	NewMarkdown            = tuilib.NewMarkdown
	NewEditor              = tuilib.NewEditor
	NewKillRing            = tuilib.NewKillRing
	NewImage               = tuilib.NewImage
	DefaultMarkdownTheme   = tuilib.DefaultMarkdownTheme
	NewKeyMap              = tuilib.NewKeyMap
	NewKeybindingsManager  = tuilib.NewKeybindingsManager
	DefaultTUIKeybindings  = tuilib.DefaultTUIKeybindings
	NewInput               = tuilib.NewInput
	FuzzyFilter            = tuilib.FuzzyFilter
	DetectCapabilities     = tuilib.DetectCapabilities
	EncodeTerminalImage    = tuilib.EncodeTerminalImage
	ImageStub              = tuilib.ImageStub
	NewProcessTerminal     = tuilib.NewProcessTerminal
	WatchResize            = tuilib.WatchResize
	MatchesKey             = tuilib.MatchesKey
)

const ImageNone = tuilib.ImageNone

type ImageRenderOptions = tuilib.ImageRenderOptions
type OverlayOptions = tuilib.OverlayOptions

var (
	ClampOverlayLines     = tuilib.ClampOverlayLines
	CompositeOverlayLines = tuilib.CompositeOverlayLines
)
