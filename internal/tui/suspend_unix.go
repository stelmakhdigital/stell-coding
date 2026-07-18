//go:build unix

package tui

import (
	"os"
	"os/signal"
	"syscall"
)

var rawModeRestore func()

func setRawModeRestore(fn func()) {
	rawModeRestore = fn
}

// suspendCmd восстанавливает cooked-режим терминала, шлёт SIGTSTP, затем снова raw mode
// после SIGCONT (shell `fg`).
func suspendCmd() Cmd {
	return func() Msg {
		if rawModeRestore != nil {
			rawModeRestore()
		}
		cont := make(chan os.Signal, 1)
		signal.Notify(cont, syscall.SIGCONT)
		_ = syscall.Kill(os.Getpid(), syscall.SIGTSTP)
		<-cont
		signal.Stop(cont)
		restore, err := EnableRawMode()
		if err == nil {
			setRawModeRestore(restore)
		}
		w, h, _ := TermSize()
		return WindowSizeMsg{Width: w, Height: h}
	}
}
