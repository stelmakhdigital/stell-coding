package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func formatToolCall(name string, args map[string]any) string {
	if args == nil {
		return name
	}
	for _, key := range []string{"command", "path", "query", "pattern", "description"} {
		if v, ok := args[key].(string); ok && strings.TrimSpace(v) != "" {
			return fmt.Sprintf("%s: %s", name, v)
		}
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, formatToolArg(args[k])))
	}
	return fmt.Sprintf("%s(%s)", name, strings.Join(parts, ", "))
}

func formatToolArg(v any) string {
	switch x := v.(type) {
	case string:
		if strings.ContainsAny(x, " \n\t") {
			return fmt.Sprintf("%q", x)
		}
		return x
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(v)
		}
		s := string(b)
		if len(s) > 120 {
			return s[:117] + "…"
		}
		return s
	}
}

func formatToolResult(name, content, errText, fullOutputPath string) string {
	body := content
	if errText != "" {
		body = "error: " + errText
		if strings.Contains(errText, "workspace trust") || strings.Contains(errText, "--approve") {
			body += " — run stell with --approve or trust the workspace"
		}
	}
	if fullOutputPath != "" && !strings.Contains(body, fullOutputPath) {
		body += fmt.Sprintf("\nFull output: %s", fullOutputPath)
	}
	return name + " → " + body
}
