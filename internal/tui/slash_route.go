package tui

import "strings"

var builtinSlashCommands = map[string]bool{
	"help": true, "h": true, "?": true,
	"hotkeys": true,
	"changelog": true,
	"quit": true, "exit": true, "q": true,
	"abort": true,
	"new": true,
	"tree": true,
	"resume": true,
	"fork": true,
	"clone": true,
	"export": true,
	"import": true,
	"copy": true,
	"share": true,
	"compact": true,
	"model": true, "models": true,
	"scoped-models": true, "scopedmodels": true,
	"theme": true,
	"themes": true,
	"session": true,
	"trust": true,
	"reload": true,
	"settings": true,
	"login": true,
	"logout": true,
	"commands": true,
	"skills": true,
	"skill": true,
	"prompts": true,
	"prompt": true,
	"pkg": true,
	"state": true,
}

// isManagedSlash — нужно ли обрабатывать ввод как slash-команду хоста,
// а не отправлять агенту. Шаблоны prompt (/name) и /skill:name разворачиваются
// в пользовательские сообщения (семантика stell).
func (m *Model) isManagedSlash(text string) bool {
	if strings.HasPrefix(text, "/skill:") {
		return false
	}
	line := strings.TrimSpace(strings.TrimPrefix(text, "/"))
	if line == "" {
		return true
	}
	cmd := strings.ToLower(strings.Fields(line)[0])
	if builtinSlashCommands[cmd] {
		return true
	}
	if findExtCommand(m.svc.ExtensionCommands(), "/"+cmd) {
		return true
	}
	if m.svc.IsPromptTemplate(cmd) {
		return false
	}
	return false
}
