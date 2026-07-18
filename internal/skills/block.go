package skills

import (
	"path/filepath"
	"regexp"
	"strings"
)

// SkillBlock — разобранный вызов skill в сообщении пользователя.
type SkillBlock struct {
	Name        string
	Location    string
	Content     string
	UserMessage string
}

var skillBlockRe = regexp.MustCompile(`(?s)^<skill name="([^"]+)" location="([^"]+)">\n(.*)\n</skill>(?:\n\n(.*))?$`)

// ParseSkillBlock извлекает skill-блок из текста сообщения (семантика stell).
func ParseSkillBlock(text string) (*SkillBlock, bool) {
	m := skillBlockRe.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return nil, false
	}
	user := strings.TrimSpace(m[4])
	return &SkillBlock{
		Name:        m[1],
		Location:    m[2],
		Content:     m[3],
		UserMessage: user,
	}, true
}

// FormatSkillBlock собирает wire-format skill-блок для сообщений пользователя.
func FormatSkillBlock(name, location, baseDir, body, userMessage string) string {
	body = strings.TrimSpace(body)
	block := `<skill name="` + escapeXMLAttr(name) + `" location="` + escapeXMLAttr(location) + `">` + "\n" +
		"References are relative to " + baseDir + ".\n\n" +
		body + "\n</skill>"
	userMessage = strings.TrimSpace(userMessage)
	if userMessage != "" {
		return block + "\n\n" + userMessage
	}
	return block
}

// FormatSkillBlockFromSkill собирает skill-блок из загруженного Skill.
func FormatSkillBlockFromSkill(s *Skill, userMessage string) string {
	baseDir := filepath.ToSlash(filepath.Dir(s.Path))
	return FormatSkillBlock(s.Name, s.Path, baseDir, s.Body, userMessage)
}
