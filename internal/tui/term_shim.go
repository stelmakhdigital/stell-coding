package tui

import tuilib "github.com/stelmakhdigital/stell-tui"

// EnableRawMode переводит stdin в raw mode; restore возвращает прежнее состояние.
func EnableRawMode() (restore func(), err error) {
	return tuilib.EnableRawMode()
}

// TermSize возвращает текущий размер терминала.
func TermSize() (w, h int, err error) {
	return tuilib.TermSize()
}

// EnableTerminalFeatures включает bracketed paste и расширенный key reporting.
func EnableTerminalFeatures() (restore func()) {
	return tuilib.EnableTerminalFeatures()
}

// QueryCellSize запрашивает у терминала размер ячейки в пикселях (CSI 16 t).
func QueryCellSize() {
	tuilib.QueryCellSize()
}

// watchSIGWINCH шлёт WindowSizeMsg при изменении размера терминала.
func watchSIGWINCH(out chan<- Msg) (stop func()) {
	return tuilib.WatchResize(func(w, h int) {
		out <- WindowSizeMsg{Width: w, Height: h}
	})
}
