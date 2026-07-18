package tui

import "testing"

func TestFormatUserErrorOllama(t *testing.T) {
	raw := `ollama: HTTP 400: {"error":"{\"error\":{\"code\":400,\"message\":\"Multimodal data provided, but model does not support multimodal requests.\",\"type\":\"invalid_request_error\"}}"}`
	got := formatUserError(raw)
	want := "ollama: Multimodal data provided, but model does not support multimodal requests."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
