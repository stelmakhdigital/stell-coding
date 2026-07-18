package catalog

import (
	"fmt"
	"strings"
)

func ParseFrontmatter(data string) (fields map[string]string, body string, err error) {
	data = strings.TrimPrefix(data, "\ufeff")
	if !strings.HasPrefix(data, "---") {
		return map[string]string{}, strings.TrimSpace(data), nil
	}
	rest := data[3:]
	rest = strings.TrimLeft(rest, "\r\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, "", fmt.Errorf("unclosed frontmatter")
	}
	fm := rest[:end]
	body = strings.TrimSpace(rest[end+4:])
	fields = map[string]string{}
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return fields, body, nil
}
