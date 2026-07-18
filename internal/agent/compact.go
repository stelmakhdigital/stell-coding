package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/stelmakhdigital/ai"
	"stell/agent/harness"
	"stell/agent/hooks"
)

type CompactInfo struct {
	RemovedMessages int    `json:"removedMessages"`
	SummaryPreview  string `json:"summaryPreview"`
}

func (s *Service) Compact(ctx context.Context) (*CompactInfo, error) {
	return s.CompactWithInstructions(ctx, "")
}

func (s *Service) CompactWithInstructions(ctx context.Context, customInstructions string) (*CompactInfo, error) {
	if s.IsStreaming() {
		return nil, ErrStreaming
	}
	msgs := s.Sessions.BuildMessages()

	// Ограничиваем keep-бюджет половиной контекстного: если компакция
	// сработала из-за превышения бюджета, сохранение большего
	// сделает её no-op с немедленным повторным срабатыванием.
	keepTokens := s.Config.Settings.Compaction.KeepRecentTokens
	if half := contextBudget(s.Model, s.Config.Settings) / 2; keepTokens <= 0 || keepTokens > half {
		keepTokens = half
	}

	hs := harness.CompactionSettings{
		Enabled:          true,
		ReserveTokens:    s.Config.Settings.Compaction.ReserveTokens,
		KeepRecentTokens: keepTokens,
	}
	prep, _ := harness.PrepareCompaction(s.Sessions.ActiveBranch(), hs)

	keepRecent := keepRecentCount(msgs, keepTokens)
	if prep != nil && prep.KeptEntries > 0 {
		// Предпочитаем точку отсечения harness, если она даёт более строгое keep-окно.
		if prep.KeptEntries < keepRecent {
			keepRecent = prep.KeptEntries
		}
	}
	if keepRecent >= len(msgs) {
		// Вся история помещается в keep-бюджет. При явном
		// /compact всё равно суммируем старую половину вместо бездействия.
		if len(msgs) <= 4 {
			return &CompactInfo{}, nil
		}
		keepRecent = len(msgs) / 2
		if keepRecent < 2 {
			keepRecent = 2
		}
	}

	toSummarize := msgs[:len(msgs)-keepRecent]
	var b strings.Builder
	for _, m := range toSummarize {
		fmt.Fprintf(&b, "%s: %s\n", m.Role, truncate(m.Content, 4000))
	}

	appendSystem := customInstructions
	if s.Hooks != nil {
		ev, _ := s.Hooks.EmitNamed(ctx, hooks.SessionBeforeCompact, s.Sessions.Header.ID, map[string]any{
			"messages": len(msgs),
			"tokens":   harness.EstimateContextTokens(msgs).Tokens,
		})
		if ev.AppendSystem != "" {
			if appendSystem != "" {
				appendSystem += "\n" + ev.AppendSystem
			} else {
				appendSystem = ev.AppendSystem
			}
		}
	}

	summary, err := s.summarize(ctx, b.String(), appendSystem)
	if err != nil {
		return nil, err
	}

	result := s.Sessions.CompactLinear(summary, keepRecent)
	if s.SessPath != "" {
		if err := s.Sessions.Save(s.SessPath); err != nil {
			return nil, err
		}
	}
	s.emitAgentHook(ctx, hooks.SessionCompact, map[string]any{
		"removed": result.Removed,
	})
	return &CompactInfo{
		RemovedMessages: result.Removed,
		SummaryPreview:  result.SummaryPreview,
	}, nil
}

func (s *Service) summarize(ctx context.Context, transcript, appendSystem string) (string, error) {
	prov, _, ok := s.Registry.Get(s.Model.Name)
	if !ok {
		return "", fmt.Errorf("model %q not registered", s.Model.Name)
	}
	system := harness.SummarizationSystemPrompt
	if appendSystem != "" {
		system += "\n\n" + appendSystem
	}
	req := ai.ChatRequest{
		Model: modelID(s.Model),
		Messages: []ai.Message{
			{Role: ai.RoleSystem, Content: system},
			{Role: ai.RoleUser, Content: transcript},
		},
		MaxTokens: 2048,
	}
	rs := s.Config.Settings.Retry
	enabled := rs.Enabled == nil || *rs.Enabled
	stream, err := chatWithRetry(ctx, prov, req, retryControl{settings: rs, enabled: enabled}, nil)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for ev := range stream {
		if ev.Type == ai.EventToken {
			out.WriteString(ev.Token)
		}
		if ev.Type == ai.EventError {
			return "", ev.Err
		}
	}
	summary := strings.TrimSpace(out.String())
	if summary == "" {
		return "Previous conversation was compacted.", nil
	}
	return summary, nil
}

// keepRecentCount возвращает, сколько хвостовых сообщений помещается в
// бюджет KeepRecentTokens (оценка), минимум 2.
func keepRecentCount(msgs []ai.Message, keepTokens int) int {
	if keepTokens <= 0 {
		keepTokens = 20000
	}
	keep := 0
	tokens := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		tokens += EstimateTokens(msgs[i : i+1])
		if tokens > keepTokens && keep >= 2 {
			break
		}
		keep++
	}
	if keep < 2 && len(msgs) >= 2 {
		twoTok := EstimateTokens(msgs[len(msgs)-2:])
		if twoTok <= keepTokens {
			keep = 2
		} else if keep < 1 {
			keep = 1
		}
	} else if keep < 1 && len(msgs) > 0 {
		keep = 1
	}
	return keep
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
