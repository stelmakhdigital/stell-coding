package prompts

import (
	"regexp"
	"strings"
)

var (
	rePositional = regexp.MustCompile(`\$([1-9][0-9]*)`)
	reAtSlice    = regexp.MustCompile(`\$\{@:([1-9][0-9]*)(?::([1-9][0-9]*))?\}`)
	reDefault    = regexp.MustCompile(`\$\{([1-9][0-9]*):-([^}]*)\}`)
)

// ParseCommandArgs разбирает args с учётом кавычек (стиль bash).
func ParseCommandArgs(argsString string) []string {
	var args []string
	var current strings.Builder
	var inQuote rune

	for _, r := range argsString {
		switch {
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '"' || r == '\'':
			inQuote = r
		case r == ' ' || r == '\t' || r == '\n':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

func Substitute(body string, args []string) string {
	out := body
	out = reDefault.ReplaceAllStringFunc(out, func(m string) string {
		sub := reDefault.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		i := atoi(sub[1])
		if i < 1 || i > len(args) || strings.TrimSpace(args[i-1]) == "" {
			return sub[2]
		}
		return args[i-1]
	})
	out = strings.ReplaceAll(out, "$ARGUMENTS", strings.Join(args, " "))
	out = strings.ReplaceAll(out, "$@", strings.Join(args, " "))
	out = reAtSlice.ReplaceAllStringFunc(out, func(m string) string {
		sub := reAtSlice.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		start := atoi(sub[1])
		if start < 1 {
			start = 1
		}
		if start > len(args) {
			return ""
		}
		if len(sub) >= 3 && sub[2] != "" {
			length := atoi(sub[2])
			end := start - 1 + length
			if end > len(args) {
				end = len(args)
			}
			return strings.Join(args[start-1:end], " ")
		}
		return strings.Join(args[start-1:], " ")
	})
	out = rePositional.ReplaceAllStringFunc(out, func(m string) string {
		sub := rePositional.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		i := atoi(sub[1])
		if i < 1 || i > len(args) {
			return ""
		}
		return args[i-1]
	})
	return out
}

// ExpandCommand разворачивает /name args, если name совпадает с загруженным шаблоном (семантика stell).
func ExpandCommand(reg *Registry, text string) string {
	if reg == nil || !strings.HasPrefix(text, "/") {
		return text
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return text
	}
	rest := strings.TrimPrefix(trimmed, "/")
	name, argsLine, _ := strings.Cut(rest, " ")
	if name == "" {
		return text
	}
	t, ok := reg.Get(name)
	if !ok {
		return text
	}
	return Substitute(t.Body, ParseCommandArgs(argsLine))
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}
