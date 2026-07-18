package packages

import (
	"path/filepath"
	"strings"
)

// MatchInclude возвращает true, если base совпадает с любым include-паттерном.
// Поддерживает отрицание с ведущим '!'.
func MatchInclude(patterns []string, base string) bool {
	if len(patterns) == 0 {
		return true
	}
	included := false
	for _, g := range patterns {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if strings.HasPrefix(g, "!") {
			neg := strings.TrimPrefix(g, "!")
			if ok, _ := filepath.Match(neg, base); ok {
				return false
			}
			continue
		}
		if ok, _ := filepath.Match(g, base); ok {
			included = true
		}
	}
	return included
}
