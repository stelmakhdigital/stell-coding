package agent

import core "stell/agent"

// Типы событий определены в stell/agent. Алиасы сохраняют стабильность call sites.
type (
	EventType     = core.EventType
	Event         = core.Event
	ToolResult    = core.ToolResult
	MessageUpdate = core.MessageUpdate
	AutoRetryInfo = core.AutoRetryInfo
)

const (
	EventToken          = core.EventToken
	EventThinkingToken  = core.EventThinkingToken
	EventMessageStart   = core.EventMessageStart
	EventMessage        = core.EventMessage
	EventMessageUpdate  = core.EventMessageUpdate
	EventToolCall       = core.EventToolCall
	EventToolCallDelta  = core.EventToolCallDelta
	EventToolProgress   = core.EventToolProgress
	EventToolResult     = core.EventToolResult
	EventDone           = core.EventDone
	EventError          = core.EventError
	EventAutoRetryStart = core.EventAutoRetryStart
	EventAutoRetryEnd   = core.EventAutoRetryEnd
	EventNotice         = core.EventNotice
	EventLabel          = core.EventLabel
)
