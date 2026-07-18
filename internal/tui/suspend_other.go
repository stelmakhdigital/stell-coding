//go:build !unix

package tui

var rawModeRestore func()

func setRawModeRestore(fn func()) {
	rawModeRestore = fn
}

func suspendCmd() Cmd {
	return func() Msg { return nil }
}
