package agent

import (
	"context"
	"fmt"
	"strings"

	"stell/agent/harness"
)

func (s *Service) MaybeBranchSummary(ctx context.Context) error {
	if !s.Config.Settings.BranchSummaryEnabled() {
		return nil
	}
	if s.Config.Settings.BranchSummarySkipPrompt() {
		return nil
	}
	leaf := s.Sessions.LeafID()
	if leaf == "" {
		return nil
	}
	msgs := s.Sessions.BuildMessages()
	if len(msgs) < 4 {
		return nil
	}
	var b strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&b, "%s: %s\n", m.Role, truncate(m.Content, 2000))
	}
	prompt := harness.BranchSummaryPrompt(b.String(), "")
	summary, err := s.summarize(ctx, prompt, "")
	if err != nil {
		return err
	}
	if _, err := s.Sessions.BranchWithSummary(leaf, summary, nil, false); err != nil {
		return err
	}
	if s.SessPath != "" {
		return s.Sessions.Save(s.SessPath)
	}
	return nil
}
