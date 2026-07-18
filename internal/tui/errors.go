package tui

import (
	"encoding/json"
	"regexp"
	"strings"
)

var ollamaHTTPError = regexp.MustCompile(`ollama: HTTP \d+: (.+)`)

func formatUserError(text string) string {
	if m := ollamaHTTPError.FindStringSubmatch(text); len(m) == 2 {
		if msg := extractNestedErrorMessage(m[1]); msg != "" {
			return "ollama: " + msg
		}
	}
	return text
}

func extractNestedErrorMessage(raw string) string {
	raw = strings.TrimSpace(raw)
	var obj any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return strings.Trim(raw, `"`)
	}
	return findErrorMessage(obj)
}

func findErrorMessage(v any) string {
	switch x := v.(type) {
	case map[string]any:
		if msg, _ := x["message"].(string); msg != "" {
			return msg
		}
		for _, key := range []string{"error", "message"} {
			if nested, ok := x[key]; ok {
				if msg := findErrorMessage(nested); msg != "" {
					return msg
				}
			}
		}
	case string:
		s := strings.TrimSpace(x)
		if strings.HasPrefix(s, "{") || strings.HasPrefix(s, `"`) {
			if msg := extractNestedErrorMessage(s); msg != "" && msg != s {
				return msg
			}
		}
		return strings.Trim(s, `"`)
	}
	return ""
}
