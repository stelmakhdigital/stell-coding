package agent

import (
	"context"
	"time"

	"github.com/stelmakhdigital/stell-ai"
)

func (s *Service) lastAssistantOnBranch() (ai.Message, bool) {
	branch := s.Sessions.ActiveBranch()
	for i := len(branch) - 1; i >= 0; i-- {
		e := branch[i]
		if e.Type == "message" && e.Message != nil && e.Message.Role == ai.RoleAssistant {
			msg := *e.Message
			ai.NormalizeMessage(&msg)
			return msg, true
		}
	}
	return ai.Message{}, false
}

func (s *Service) shouldRetryAssistantError() bool {
	if !s.AutoRetryEnabled() {
		return false
	}
	msg, ok := s.lastAssistantOnBranch()
	if !ok {
		return false
	}
	return IsRetryableAssistantMessage(msg)
}

func (s *Service) retryDelay(attempt int) time.Duration {
	rs := s.Config.Settings.Retry
	delay := time.Duration(rs.BaseDelayMs) * time.Millisecond
	if delay <= 0 {
		delay = 2 * time.Second
	}
	if attempt <= 0 {
		return delay
	}
	return delay * time.Duration(1<<(attempt-1))
}

func (s *Service) maxRetryAttempts() int {
	rs := s.Config.Settings.Retry
	if rs.MaxRetries <= 0 {
		return 3
	}
	return rs.MaxRetries
}

func (s *Service) waitAssistantRetry(ctx context.Context, events chan<- Event, attempt int) bool {
	if s.TakeAbortRetry() {
		return false
	}
	errMsg := ""
	if msg, ok := s.lastAssistantOnBranch(); ok {
		errMsg = msg.ErrorMessage
	}
	maxAttempts := s.maxRetryAttempts()
	delay := s.retryDelay(attempt)
	emit(events, Event{
		Type: EventAutoRetryStart,
		WillRetry: true,
		AutoRetry: &AutoRetryInfo{
			Attempt:      attempt,
			MaxAttempts:  maxAttempts,
			DelayMs:      int(delay.Milliseconds()),
			ErrorMessage: errMsg,
			WillRetry:    true,
		},
	})
	select {
	case <-ctx.Done():
		emit(events, Event{
			Type: EventAutoRetryEnd,
			WillRetry: false,
			AutoRetry: &AutoRetryInfo{Attempt: attempt, Success: false, WillRetry: false, FinalError: ctx.Err().Error()},
		})
		return false
	case <-time.After(delay):
	}
	if s.TakeAbortRetry() {
		emit(events, Event{
			Type: EventAutoRetryEnd,
			WillRetry: false,
			AutoRetry: &AutoRetryInfo{Attempt: attempt, Success: false, WillRetry: false},
		})
		return false
	}
	emit(events, Event{
		Type: EventAutoRetryEnd,
		WillRetry: true,
		AutoRetry: &AutoRetryInfo{Attempt: attempt, Success: true, WillRetry: true},
	})
	return true
}
