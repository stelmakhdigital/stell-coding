package skills

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FormatForPrompt рендерит XML-каталог Agent Skills для system
// prompt. Skills с disable-model-invocation пропускаются (вызов через /skill:name).
func FormatForPrompt(skills []*Skill) string {
	var visible []*Skill
	for _, s := range skills {
		if s != nil && !s.DisableModelInvocation {
			visible = append(visible, s)
		}
	}
	if len(visible) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\nThe following skills provide specialized instructions for specific tasks.\n")
	b.WriteString("Use the read tool to load a skill's file when the task matches its description.\n")
	b.WriteString("When a skill file references a relative path, resolve it against the skill directory (parent of SKILL.md / dirname of the path) and use that absolute path in tool commands.\n\n")
	b.WriteString("<available_skills>\n")
	for _, s := range visible {
		loc := filepath.ToSlash(s.Path)
		b.WriteString("  <skill>\n")
		fmt.Fprintf(&b, "    <name>%s</name>\n", escapeXML(s.Name))
		fmt.Fprintf(&b, "    <description>%s</description>\n", escapeXML(s.Description))
		fmt.Fprintf(&b, "    <location>%s</location>\n", escapeXML(loc))
		b.WriteString("  </skill>\n")
	}
	b.WriteString("</available_skills>")
	return b.String()
}

// ExpandCommand разворачивает /skill:name args в skill-блок сообщения пользователя (семантика stell).
func ExpandCommand(reg *Registry, text string) string {
	if reg == nil || !strings.HasPrefix(text, "/skill:") {
		return text
	}
	space := strings.Index(text, " ")
	skillName := text
	args := ""
	if space >= 0 {
		skillName = text[:space]
		args = strings.TrimSpace(text[space+1:])
	}
	skillName = strings.TrimPrefix(skillName, "/skill:")
	if skillName == "" {
		return text
	}
	s, ok := reg.Get(skillName)
	if !ok {
		return text
	}
	return FormatSkillBlockFromSkill(s, args)
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func escapeXMLAttr(s string) string {
	s = escapeXML(s)
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
