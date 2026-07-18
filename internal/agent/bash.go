package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/stelmakhdigital/stell-agent/hooks"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-agent/tools"
)

// RunBashOptions настраивает пользовательский bash (! / !! в TUI или RPC bash).
type RunBashOptions struct {
	ExcludeFromContext bool
	OnProgress         func(string)
}

type pendingBashRecord struct {
	command string
	output  string
	meta    session.BashEntryMeta
}

// IsBashRunning сообщает, выполняется ли пользовательская bash-команда.
func (s *Service) IsBashRunning() bool {
	s.bashMu.Lock()
	defer s.bashMu.Unlock()
	return s.bashCancel != nil
}

// RunBash выполняет shell-команду в workspace без хода агента.
func (s *Service) RunBash(ctx context.Context, command string, opts RunBashOptions) (tools.BashResult, error) {
	if command == "" {
		return tools.BashResult{}, fmt.Errorf("command required")
	}
	s.bashMu.Lock()
	if s.bashCancel != nil {
		s.bashMu.Unlock()
		return tools.BashResult{}, ErrBashRunning
	}
	s.bashMu.Unlock()

	if s.Hooks != nil {
		ev := &hooks.Event{
			Name:      hooks.UserBash,
			SessionID: s.Sessions.Header.ID,
			Payload: map[string]any{
				"command":            command,
				"excludeFromContext": opts.ExcludeFromContext,
			},
			Command: command,
		}
		if err := s.Hooks.Emit(ctx, ev); err != nil {
			return tools.BashResult{}, err
		}
		if ev.Cancel {
			return tools.BashResult{}, fmt.Errorf("cancelled by extension hook")
		}
		if strings.TrimSpace(ev.Command) != "" {
			command = ev.Command
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	if opts.OnProgress != nil {
		ctx = tools.WithProgress(ctx, opts.OnProgress)
	}
	s.bashMu.Lock()
	s.bashCancel = cancel
	s.bashMu.Unlock()
	defer func() {
		s.bashMu.Lock()
		s.bashCancel = nil
		s.bashMu.Unlock()
		cancel()
	}()

	res, err := s.Tools.RunBash(ctx, command)
	if err != nil {
		return tools.BashResult{}, err
	}

	meta := session.BashEntryMeta{
		ExcludeFromContext: opts.ExcludeFromContext,
		ExitCode:           res.ExitCode,
		Cancelled:          res.Cancelled,
		Truncated:          res.Truncated,
	}
	if s.IsStreaming() {
		s.mu.Lock()
		s.pendingBash = append(s.pendingBash, pendingBashRecord{
			command: command,
			output:  res.Output,
			meta:    meta,
		})
		s.mu.Unlock()
	} else {
		s.recordBashEntry(command, res.Output, meta)
	}
	return res, nil
}

func (s *Service) recordBashEntry(command, output string, meta session.BashEntryMeta) {
	_, _ = s.Sessions.AppendBashEntry(command, output, meta)
	if s.SessPath != "" {
		_ = s.Sessions.Save(s.SessPath)
	}
}

// FlushPendingBash записывает отложенные bash entry после завершения хода агента.
func (s *Service) FlushPendingBash() {
	s.mu.Lock()
	pending := s.pendingBash
	s.pendingBash = nil
	s.mu.Unlock()
	for _, rec := range pending {
		s.recordBashEntry(rec.command, rec.output, rec.meta)
	}
}
