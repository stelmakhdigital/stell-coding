package agent

import (
	"context"

	"stell/coding-agent/internal/discovery"
	"stell/coding-agent/internal/extensions"
)

func (s *Service) ReloadExtensions(ctx context.Context) ([]extensions.ReloadStatus, error) {
	if s.Extensions == nil {
		return nil, nil
	}
	if cat, err := discovery.Load(s.Config); err == nil {
		s.Catalog = cat
	}
	return s.Extensions.Reload(ctx)
}

func (s *Service) InvokeExtensionCommand(ctx context.Context, name string, args []string) (extensions.CommandResult, error) {
	if s.Extensions == nil {
		return extensions.CommandResult{}, ErrNoExtensions
	}
	return s.Extensions.InvokeCommand(ctx, name, args, s.Sessions.Header.ID)
}

func (s *Service) ExtensionCommands() []extensions.CommandEntry {
	if s.Extensions == nil {
		return nil
	}
	return s.Extensions.Commands()
}
