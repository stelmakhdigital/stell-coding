package config

import "testing"

func TestValidateAcceptsOpenAIResponses(t *testing.T) {
	mf := ModelsFile{
		Models: []ModelConfig{{
			Name: "gpt", Provider: "openai", Model: "gpt-5", APIType: "openai-responses",
		}},
	}
	if err := mf.Validate(); err != nil {
		t.Fatalf("openai-responses should be accepted: %v", err)
	}
}
