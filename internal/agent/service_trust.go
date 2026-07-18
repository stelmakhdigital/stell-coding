package agent

import (
	"context"

	"github.com/stelmakhdigital/stell-agent/hooks"
)

func (s *Service) EmitProjectTrustHook(ctx context.Context, workspace string, trusted bool) {
	s.emitAgentHook(ctx, hooks.ProjectTrust, map[string]any{
		"workspace": workspace,
		"trusted":   trusted,
	})
}
