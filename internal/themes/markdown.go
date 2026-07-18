package themes

import (
	"fmt"
	"strconv"
	"strings"
)

// ColorToANSI конвертирует цвет темы (#RRGGBB, индекс 0-255 или пусто) в ANSI SGR-последовательность.
func ColorToANSI(v string, bg bool) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	code := 38
	if bg {
		code = 48
	}
	if strings.HasPrefix(v, "#") && len(v) == 7 {
		r, _ := strconv.ParseInt(v[1:3], 16, 0)
		g, _ := strconv.ParseInt(v[3:5], 16, 0)
		b, _ := strconv.ParseInt(v[5:7], 16, 0)
		return fmt.Sprintf("\x1b[%d;2;%d;%d;%dm", code, r, g, b)
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 255 {
		return fmt.Sprintf("\x1b[%d;5;%dm", code, n)
	}
	return ""
}

// MarkdownThemeColors хранит ANSI-последовательности для рендера markdown (поля MarkdownTheme в stell/tui).
type MarkdownThemeColors struct {
	Heading, Link, LinkURL, Code, CodeBlock, CodeBlockBorder string
	Quote, QuoteBorder, HR, ListBullet, Strike, Bold, Italic string
	Reset                                                    string
	SyntaxComment, SyntaxKeyword, SyntaxFunction             string
	SyntaxVariable, SyntaxString, SyntaxNumber               string
	SyntaxType, SyntaxOperator, SyntaxPunctuation            string
}

// MarkdownTheme маппит токены темы md* и syntax* в ANSI-последовательности.
func (t Theme) MarkdownTheme() MarkdownThemeColors {
	fg := func(key string) string {
		return ColorToANSI(t.Color(key, ""), false)
	}
	return MarkdownThemeColors{
		Heading:         fg("mdHeading"),
		Link:            fg("mdLink"),
		LinkURL:         fg("mdLinkUrl"),
		Code:            fg("mdCode"),
		CodeBlock:       fg("mdCodeBlock"),
		CodeBlockBorder: fg("mdCodeBlockBorder"),
		Quote:           fg("mdQuote"),
		QuoteBorder:     fg("mdQuoteBorder"),
		HR:              fg("mdHr"),
		ListBullet:      fg("mdListBullet"),
		Strike:          "\x1b[9m",
		Bold:            "\x1b[1m",
		Italic:          "\x1b[3m",
		Reset:           "\x1b[0m",
		SyntaxComment:   fg("syntaxComment"),
		SyntaxKeyword:   fg("syntaxKeyword"),
		SyntaxFunction:  fg("syntaxFunction"),
		SyntaxVariable:  fg("syntaxVariable"),
		SyntaxString:    fg("syntaxString"),
		SyntaxNumber:    fg("syntaxNumber"),
		SyntaxType:      fg("syntaxType"),
		SyntaxOperator:  fg("syntaxOperator"),
		SyntaxPunctuation: fg("syntaxPunctuation"),
	}
}

// HighlightCode применяет минимальный токен-хайлайтер по цветам syntax*.
// Распознаёт строки, комментарии (// #), числа и типичные keywords; остальное — цвет CodeBlock.
func HighlightCode(src string, md MarkdownThemeColors) string {
	if src == "" {
		return src
	}
	reset := md.Reset
	if reset == "" {
		reset = "\x1b[0m"
	}
	base := md.CodeBlock
	var b strings.Builder
	i := 0
	for i < len(src) {
		// построчный комментарий
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				end = len(src) - i
			}
			b.WriteString(md.SyntaxComment)
			b.WriteString(src[i : i+end])
			b.WriteString(reset)
			i += end
			continue
		}
		if src[i] == '#' && (i == 0 || src[i-1] == '\n' || src[i-1] == ' ') {
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				end = len(src) - i
			}
			b.WriteString(md.SyntaxComment)
			b.WriteString(src[i : i+end])
			b.WriteString(reset)
			i += end
			continue
		}
		// строка
		if src[i] == '"' || src[i] == '\'' || src[i] == '`' {
			q := src[i]
			j := i + 1
			for j < len(src) {
				if src[j] == '\\' && j+1 < len(src) {
					j += 2
					continue
				}
				if src[j] == q {
					j++
					break
				}
				j++
			}
			b.WriteString(md.SyntaxString)
			b.WriteString(src[i:j])
			b.WriteString(reset)
			i = j
			continue
		}
		// число
		if src[i] >= '0' && src[i] <= '9' {
			j := i + 1
			for j < len(src) && ((src[j] >= '0' && src[j] <= '9') || src[j] == '.' || src[j] == 'x' || src[j] == 'X') {
				j++
			}
			b.WriteString(md.SyntaxNumber)
			b.WriteString(src[i:j])
			b.WriteString(reset)
			i = j
			continue
		}
		// идентификатор / keyword
		if isIdentStart(src[i]) {
			j := i + 1
			for j < len(src) && isIdentCont(src[j]) {
				j++
			}
			word := src[i:j]
			color := base
			if isKeyword(word) {
				color = md.SyntaxKeyword
			}
			if color != "" {
				b.WriteString(color)
				b.WriteString(word)
				b.WriteString(reset)
			} else {
				b.WriteString(word)
			}
			i = j
			continue
		}
		if base != "" && (src[i] == '+' || src[i] == '-' || src[i] == '=' || src[i] == '<' || src[i] == '>' || src[i] == '!' || src[i] == '&' || src[i] == '|') {
			b.WriteString(md.SyntaxOperator)
			b.WriteByte(src[i])
			b.WriteString(reset)
			i++
			continue
		}
		if base != "" {
			b.WriteString(base)
			b.WriteByte(src[i])
			b.WriteString(reset)
		} else {
			b.WriteByte(src[i])
		}
		i++
	}
	return b.String()
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentCont(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func isKeyword(w string) bool {
	switch w {
	case "func", "return", "if", "else", "for", "range", "switch", "case", "default",
		"type", "struct", "interface", "map", "chan", "go", "defer", "package", "import",
		"var", "const", "true", "false", "nil", "error", "string", "int", "bool",
		"function", "let", "class", "def", "from", "as", "with", "async", "await",
		"public", "private", "static", "void", "new", "this", "self":
		return true
	}
	return false
}
