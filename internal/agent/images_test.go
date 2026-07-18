package agent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/config"
)

func TestBuildUserMessageVision(t *testing.T) {
	imgs := []ai.ImageContent{{Type: "image", Data: "abc", MimeType: "image/png"}}
	msg := BuildUserMessage("hi", imgs, config.ModelConfig{Provider: "openai", Input: []string{"text", "image"}})
	if len(msg.Images) != 1 {
		t.Fatalf("expected images on message, got %d", len(msg.Images))
	}
	if msg.Content != "hi" {
		t.Fatalf("expected text preserved, got %q", msg.Content)
	}
}

func TestBuildUserMessageOpenRouterVision(t *testing.T) {
	imgs := []ai.ImageContent{{Type: "image", Data: "abc", MimeType: "image/png"}}
	msg := BuildUserMessage("hi", imgs, config.ModelConfig{Provider: "openrouter", Input: []string{"text", "image"}})
	if len(msg.Images) != 1 {
		t.Fatalf("openrouter+image input should attach images, got %d", len(msg.Images))
	}
}

func TestBuildUserMessageTextOnlyModel(t *testing.T) {
	imgs := []ai.ImageContent{{Type: "image", Data: "abc", MimeType: "image/png"}}
	msg := BuildUserMessage("hi", imgs, config.ModelConfig{Provider: "openai", Input: []string{"text"}})
	if len(msg.Images) != 0 {
		t.Fatal("text-only model should not attach images")
	}
	if !strings.Contains(msg.Content, "[image 1:") {
		t.Fatalf("expected placeholder in content, got %q", msg.Content)
	}
}

func TestBuildUserMessageUnknownInputUsesPlaceholder(t *testing.T) {
	imgs := []ai.ImageContent{{Type: "image", Data: "abc", MimeType: "image/png"}}
	msg := BuildUserMessage("hi", imgs, config.ModelConfig{Provider: "ollama", Name: "local-ornith"})
	if len(msg.Images) != 0 {
		t.Fatal("model without input[] should not attach images")
	}
	if !strings.Contains(msg.Content, "[image 1:") {
		t.Fatalf("expected placeholder in content, got %q", msg.Content)
	}
}

func TestIsMultimodalUnsupportedError(t *testing.T) {
	err := fmt.Errorf("ollama: HTTP 400: model does not support multimodal requests")
	if !IsMultimodalUnsupportedError(err) {
		t.Fatal("expected multimodal error detection")
	}
}

func TestBuildUserMessageMockFallback(t *testing.T) {
	imgs := []ai.ImageContent{{Type: "image", Data: "abc", MimeType: "image/png"}}
	msg := BuildUserMessage("hi", imgs, config.ModelConfig{Provider: "mock"})
	if len(msg.Images) != 0 {
		t.Fatal("mock should not attach images")
	}
	if !strings.Contains(msg.Content, "[image 1:") {
		t.Fatalf("expected placeholder in content, got %q", msg.Content)
	}
}

func TestIsImagePath(t *testing.T) {
	if !IsImagePath("a.png") || IsImagePath("a.go") {
		t.Fatal("image path detection failed")
	}
}

func TestPromptWithImages(t *testing.T) {
	imgs := []ai.ImageContent{{Type: "image", Data: "abc", MimeType: "image/png"}}
	if got := PromptWithImages("", imgs); got != DefaultImagePrompt {
		t.Fatalf("expected default prompt, got %q", got)
	}
	if got := PromptWithImages("hello", imgs); got != "hello" {
		t.Fatalf("expected text preserved, got %q", got)
	}
	if got := PromptWithImages("", nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
