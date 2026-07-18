package agent

import (
	"context"
	"strings"
	"time"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
)

type retryControl struct {
	settings    config.RetrySettings
	enabled     bool
	shouldAbort func() bool
}

type retryHook func(start bool, info AutoRetryInfo)

func chatWithRetry(ctx context.Context, prov ai.Provider, req ai.ChatRequest, ctrl retryControl, hook retryHook) (<-chan ai.ChatEvent, error) {
	rs := ctrl.settings
	enabled := ctrl.enabled
	maxRetries := rs.MaxRetries
	if maxRetries < 0 {
		maxRetries = 3
	}
	delay := time.Duration(rs.BaseDelayMs) * time.Millisecond
	if delay <= 0 {
		delay = 2 * time.Second
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctrl.shouldAbort != nil && ctrl.shouldAbort() {
			return nil, errRetryAborted
		}
		stream, err := prov.Chat(ctx, req)
		if err == nil {
			if !enabled {
				return stream, nil
			}
			return relayWithRetry(ctx, stream, prov, req, rs, attempt, enabled, ctrl.shouldAbort, hook), nil
		}
		lastErr = err
		if !enabled || !isRetryable(err) || attempt == maxRetries {
			return nil, err
		}
		if hook != nil {
			hook(true, AutoRetryInfo{
				Attempt:      attempt + 1,
				MaxAttempts:  maxRetries,
				DelayMs:      int((delay * time.Duration(1<<attempt)).Milliseconds()),
				ErrorMessage: err.Error(),
				WillRetry:    attempt < maxRetries,
			})
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay * time.Duration(1<<attempt)):
		}
		if ctrl.shouldAbort != nil && ctrl.shouldAbort() {
			return nil, errRetryAborted
		}
		if hook != nil {
			hook(false, AutoRetryInfo{Attempt: attempt + 1, WillRetry: false, Success: true})
		}
	}
	return nil, lastErr
}

var errRetryAborted = configErr("retry aborted")

type configErr string

func (e configErr) Error() string { return string(e) }

func relayWithRetry(ctx context.Context, stream <-chan ai.ChatEvent, prov ai.Provider, req ai.ChatRequest, rs config.RetrySettings, startAttempt int, enabled bool, shouldAbort func() bool, hook retryHook) <-chan ai.ChatEvent {
	out := make(chan ai.ChatEvent, 64)
	go func() {
		defer close(out)
		attempt := startAttempt
		current := stream
		for {
			gotError := false
			var errEv ai.ChatEvent
			for ev := range current {
				if ev.Type == ai.EventError && enabled && isRetryable(ev.Err) {
					gotError = true
					errEv = ev
					break
				}
				out <- ev
			}
			if !gotError {
				return
			}
			maxRetries := rs.MaxRetries
			if maxRetries < 0 {
				maxRetries = 3
			}
			if attempt >= maxRetries {
				out <- errEv
				return
			}
			if shouldAbort != nil && shouldAbort() {
				out <- ai.ChatEvent{Type: ai.EventError, Err: errRetryAborted}
				return
			}
			if hook != nil {
				errMsg := ""
				if errEv.Err != nil {
					errMsg = errEv.Err.Error()
				}
				maxRetries := rs.MaxRetries
				if maxRetries < 0 {
					maxRetries = 3
				}
				delay := time.Duration(rs.BaseDelayMs) * time.Millisecond
				if delay <= 0 {
					delay = 2 * time.Second
				}
				hook(true, AutoRetryInfo{
					Attempt:      attempt + 1,
					MaxAttempts:  maxRetries,
					DelayMs:      int((delay * time.Duration(1<<attempt)).Milliseconds()),
					ErrorMessage: errMsg,
					WillRetry:    attempt < maxRetries,
				})
			}
			delay := time.Duration(rs.BaseDelayMs) * time.Millisecond
			if delay <= 0 {
				delay = 2 * time.Second
			}
			select {
			case <-ctx.Done():
				out <- ai.ChatEvent{Type: ai.EventError, Err: ctx.Err()}
				return
			case <-time.After(delay * time.Duration(1<<attempt)):
			}
			if shouldAbort != nil && shouldAbort() {
				out <- ai.ChatEvent{Type: ai.EventError, Err: errRetryAborted}
				return
			}
			if hook != nil {
				hook(false, AutoRetryInfo{
					Attempt: attempt + 1,
					Success: true,
					WillRetry: true,
				})
			}
			attempt++
			next, err := prov.Chat(ctx, req)
			if err != nil {
				out <- ai.ChatEvent{Type: ai.EventError, Err: err}
				return
			}
			current = next
		}
	}()
	return out
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	return isRetryableMessage(err.Error())
}

// IsRetryableAssistantMessage сообщает, является ли ошибка assistant повторяемой.
func IsRetryableAssistantMessage(msg ai.Message) bool {
	if msg.Role != ai.RoleAssistant || msg.StopReason != "error" || msg.ErrorMessage == "" {
		return false
	}
	return isRetryableMessage(msg.ErrorMessage)
}

func isRetryableMessage(msg string) bool {
	lower := strings.ToLower(msg)
	for _, s := range []string{
		"429", "rate limit", "500", "502", "503", "504", "timeout", "temporarily",
		"empty stream", "ended without", "no input tokens",
		"overloaded", "too many requests", "service unavailable", "server error",
		"internal error", "connection error", "connection refused", "fetch failed",
		"terminated", "retry delay",
	} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
