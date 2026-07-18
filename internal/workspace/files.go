package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

const defaultIndexLimit = 8000

// ListFiles обходит workspace и возвращает относительные пути (.git пропускается).
func ListFiles(root string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = defaultIndexLimit
	}
	root = filepath.Clean(root)
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil
		}
		out = append(out, rel)
		if len(out) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	return out, err
}

// FuzzyFilter ранжирует пути по subsequence-совпадению с query.
func FuzzyFilter(paths []string, query string, limit int) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		if limit > 0 && len(paths) > limit {
			return append([]string(nil), paths[:limit]...)
		}
		return append([]string(nil), paths...)
	}
	type scored struct {
		path  string
		score int
	}
	var matches []scored
	for _, p := range paths {
		if s, ok := fuzzyScore(strings.ToLower(p), q); ok {
			matches = append(matches, scored{path: p, score: s})
		}
	}
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].score > matches[i].score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
	if limit <= 0 {
		limit = 40
	}
	n := len(matches)
	if n > limit {
		n = limit
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, matches[i].path)
	}
	return out
}

func fuzzyScore(path, query string) (int, bool) {
	if strings.Contains(path, query) {
		return 1000 - len(path), true
	}
	qi := 0
	score := 0
	for _, ch := range path {
		if qi < len(query) && query[qi] == byte(ch) {
			score += 10
			qi++
		}
	}
	if qi != len(query) {
		return 0, false
	}
	return score, true
}
