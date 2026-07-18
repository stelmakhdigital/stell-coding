package tui

import (
	"context"
	"io"
	"os"
	"time"

	"stell/coding-agent/internal/themes"
	tuilib "stell/tui"
)

func runInteractive(ctx context.Context, opts Options) error {
	m := NewModel(ctx, opts)
	ui := New(os.Stdout, true)
	root := &appRoot{m: &m}
	ui.SetRoot(NewContainer(root))
	ui.SetFocus(m.composer.ed)

	restore, err := EnableRawMode()
	if err != nil {
		return err
	}
	setRawModeRestore(restore)
	defer restore()
	defer setRawModeRestore(nil)
	restoreFeat := EnableTerminalFeatures()
	defer restoreFeat()
	QueryCellSize()
	if m.cfg != nil {
		ui.SetShowHardwareCursor(m.cfg.Settings.HardwareCursorEnabled())
		ui.SetShowImages(m.cfg.Settings.ImagesEnabled())
		if m.cfg.Settings.DiffScrollEnabled() {
			ui.SetDiffStrategy(DiffScroll)
		}
		if m.composer.ed != nil {
			m.composer.ed.SetShowHardwareCursor(m.cfg.Settings.HardwareCursorEnabled())
			m.composer.ed.SetPaddingX(m.cfg.Settings.EditorPaddingX)
		}
	}

	if w, h, err := TermSize(); err == nil {
		m, _ = m.Update(WindowSizeMsg{Width: w, Height: h})
		root.m = &m
		ui.SetSize(w, h)
		ui.SetRoot(NewContainer(root))
		ui.SetFocus(m.composer.ed)
	}

	msgCh := make(chan Msg, 64)
	if m.themeCtrl != nil && m.cfg != nil {
		themeCh := make(chan themes.Theme, 1)
		m.themeCtrl = themes.NewController(themes.ResolveOpts{
			GlobalDir:  m.cfg.GlobalDir,
			ProjectDir: m.cfg.ProjectDir,
			Workspace:  m.cfg.Workspace,
		}, func(th themes.Theme) {
			select {
			case themeCh <- th:
			default:
			}
		})
		applied := m.themeCtrl.ApplyFromSettings(m.cfg.Settings.Theme)
		m.activeTheme = applied
		m.colors = paletteFromTheme(applied)
		m.themeCtrl.StartHotReload()
		defer m.themeCtrl.StopHotReload()
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case th := <-themeCh:
					select {
					case msgCh <- themeReloadMsg{theme: th}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}
	var schedule func(Cmd)
	schedule = func(cmd Cmd) {
		if cmd == nil {
			return
		}
		go func() {
			msg := cmd()
			if msg == nil {
				return
			}
			if bm, ok := msg.(batchMsg); ok {
				for _, c := range bm.cmds {
					schedule(c)
				}
				return
			}
			select {
			case msgCh <- msg:
			case <-ctx.Done():
			}
		}()
	}

	apply := func(msg Msg) (quit bool) {
		if msg == nil {
			return false
		}
		if _, ok := msg.(quitMsg); ok {
			return true
		}
		if bm, ok := msg.(batchMsg); ok {
			for _, c := range bm.cmds {
				schedule(c)
			}
			return false
		}
		var next Cmd
		m, next = m.Update(msg)
		root.m = &m
		ui.SetRoot(NewContainer(root))
		if m.forceFullRedraw {
			m.forceFullRedraw = false
			ui.ForceFullRedraw()
		}
		if m.overlay == "" && m.extReplaceEditor == "" {
			ui.SetFocus(m.composer.ed)
		}
		ui.RequestRender()
		schedule(next)
		return false
	}

	schedule(m.Init())
	_ = ui.RenderNow()

	inputCh := make(chan string, 32)
	errCh := make(chan error, 1)
	go readInput(ctx, os.Stdin, inputCh, errCh)

	stopResize := watchSIGWINCH(msgCh)
	defer stopResize()

	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	defer func() { _ = ui.Close() }()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil && err != io.EOF {
				return err
			}
			return nil
		case data := <-inputCh:
			if handleCellSizeReply(data, &m, ui) {
				continue
			}
			if apply(parseKey(data)) {
				return nil
			}
			_ = ui.RenderNow()
		case msg := <-msgCh:
			if ws, ok := msg.(WindowSizeMsg); ok {
				ui.SetSize(ws.Width, ws.Height)
			}
			if apply(msg) {
				return nil
			}
			_ = ui.RenderNow()
		case <-ticker.C:
			if apply(tickMsg{}) {
				return nil
			}
			_ = ui.RenderNow()
		}
	}
}

func readInput(ctx context.Context, r io.Reader, ch chan<- string, errCh chan<- error) {
	buf := make([]byte, 4096)
	stdinBuf := tuilib.NewStdinBuffer(tuilib.StdinBufferOptions{})
	stdinBuf.OnData = func(data string) {
		sendInput(ctx, ch, data)
	}
	stdinBuf.OnPaste = func(content string) {
		sendInput(ctx, ch, "\x1b[200~"+content+"\x1b[201~")
	}
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			if len(buf[:n]) == 1 && buf[0] > 127 {
				chunk = "\x1b" + string(rune(buf[0]-128))
			}
			stdinBuf.Process(chunk)
		}
		if err != nil {
			select {
			case errCh <- err:
			default:
			}
			return
		}
	}
}

func sendInput(ctx context.Context, ch chan<- string, data string) {
	if data == "" {
		return
	}
	select {
	case ch <- data:
	case <-ctx.Done():
	}
}
