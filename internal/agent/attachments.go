package agent

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/stelmakhdigital/stell-agent/tools"
)

var atPathRe = regexp.MustCompile(`@([^\s@]+)`)

// ExpandAttachments читает ссылки @path в сообщении и дописывает содержимое файлов.
func ExpandAttachments(workspace, message string, extra []string) (string, error) {
	seen := map[string]bool{}
	var paths []string
	for _, p := range extra {
		if p != "" && !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for _, m := range atPathRe.FindAllStringSubmatch(message, -1) {
		if len(m) < 2 {
			continue
		}
		p := m[1]
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return message, nil
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(message))
	for _, rel := range paths {
		abs, _, err := tools.ResolvePath(workspace, rel)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return "", fmt.Errorf("attachment %s: %w", rel, err)
		}
		content := string(data)
		if len(content) > 64000 {
			content = content[:64000] + "\n… (truncated)"
		}
		fmt.Fprintf(&b, "\n\n[Attached file: %s]\n```\n%s\n```", rel, content)
	}
	return b.String(), nil
}
