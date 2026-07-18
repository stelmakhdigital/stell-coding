package tui

// KeyDisplay возвращает первую привязанную клавишу действия (для подсказок /help).
func KeyDisplay(keys Keybindings, action string) string {
	parts := splitBindingKeys(keys.Bindings[action])
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
