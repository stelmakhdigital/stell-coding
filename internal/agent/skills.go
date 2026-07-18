package agent

import (
	"fmt"

	"github.com/stelmakhdigital/stell-coding/internal/prompts"
	"github.com/stelmakhdigital/stell-coding/internal/skills"
)

func (s *Service) RenderPrompt(name string, args []string) (string, error) {
	if s.Catalog == nil || s.Catalog.Prompts == nil {
		return "", fmt.Errorf("prompts not loaded")
	}
	return s.Catalog.Prompts.Render(name, args)
}

func (s *Service) IsPromptTemplate(name string) bool {
	return s.Catalog != nil && s.Catalog.Prompts != nil && s.Catalog.Prompts.Has(name)
}

// PrepareMessage раскрывает команды /skill:name и /template в текст user-сообщения (семантика stell).
func (s *Service) PrepareMessage(message string) string {
	if s == nil {
		return message
	}
	skillCommands := true
	if s.Config != nil && s.Config.Settings.EnableSkillCommands != nil {
		skillCommands = *s.Config.Settings.EnableSkillCommands
	}
	if skillCommands && s.Catalog != nil && s.Catalog.Skills != nil {
		message = skills.ExpandCommand(s.Catalog.Skills, message)
	}
	if s.Catalog != nil && s.Catalog.Prompts != nil {
		message = prompts.ExpandCommand(s.Catalog.Prompts, message)
	}
	return message
}
