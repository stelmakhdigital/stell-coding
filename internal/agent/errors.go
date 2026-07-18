package agent

import "errors"

var (
	ErrStreaming    = errors.New("agent is already streaming")
	ErrNotStreaming = errors.New("agent is not streaming")
	ErrNoExtensions = errors.New("extensions not loaded")
	ErrBashRunning  = errors.New("bash command already running")
)
