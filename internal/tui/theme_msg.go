package tui

import "github.com/stelmakhdigital/stell-coding/internal/themes"

type themeReloadMsg struct {
	theme themes.Theme
}
