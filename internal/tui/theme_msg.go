package tui

import "stell/coding-agent/internal/themes"

type themeReloadMsg struct {
	theme themes.Theme
}
