package themes

import "strings"

// ParseAutoThemeSetting разбирает "lightTheme/darkTheme" (ровно один '/').
// Возвращает ok=false, если setting — фиксированное имя темы или пусто.
func ParseAutoThemeSetting(setting string) (light, dark string, ok bool) {
	setting = strings.TrimSpace(setting)
	if setting == "" || strings.Count(setting, "/") != 1 {
		return "", "", false
	}
	parts := strings.SplitN(setting, "/", 2)
	light, dark = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if light == "" || dark == "" || strings.Contains(light, "/") || strings.Contains(dark, "/") {
		return "", "", false
	}
	return light, dark, true
}

// ResolveThemeSetting возвращает конкретное имя темы для setting.
// Auto "light/dark" выбирает по terminalTheme ("light" или "dark").
func ResolveThemeSetting(setting, terminalTheme string) string {
	setting = strings.TrimSpace(setting)
	if setting == "" {
		return ""
	}
	light, dark, auto := ParseAutoThemeSetting(setting)
	if !auto {
		return setting
	}
	if terminalTheme == "light" {
		return light
	}
	return dark
}
