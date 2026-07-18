package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Settings   Settings
	Models     []ModelConfig
	GlobalDir  string
	ProjectDir string // workspace/.stell, если существует
	Auth       *Auth
	Workspace  string
}

func GlobalDir() (string, error) {
	if dir := os.Getenv("STELL_AGENT_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".stell", "agent"), nil
}

func Load(workspace string) (*Config, error) {
	globalDir, err := GlobalDir()
	if err != nil {
		return nil, err
	}
	settings := DefaultSettings()
	globalSettingsPath := filepath.Join(globalDir, "settings.json")
	if p := os.Getenv("STELL_CONFIG"); p != "" {
		globalSettingsPath = p
	}
	if s, ok, err := readSettings(globalSettingsPath); err != nil {
		return nil, err
	} else if ok {
		settings = mergeSettings(settings, s)
	}

	projectStell := filepath.Join(workspace, ".stell")
	projectDir := ""
	if info, err := os.Stat(projectStell); err == nil && info.IsDir() {
		projectDir = projectStell
		if s, ok, err := readSettings(filepath.Join(projectStell, "settings.json")); err != nil {
			return nil, err
		} else if ok {
			settings = mergeSettings(settings, s)
		}
	}

	if err := settings.Validate(); err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}

	var models ModelsFile
	if m, ok, err := readModels(filepath.Join(globalDir, "models.json")); err != nil {
		return nil, err
	} else if ok {
		models = m
	}
	if projectDir != "" {
		if pm, ok, err := readModels(filepath.Join(projectDir, "models.json")); err != nil {
			return nil, err
		} else if ok {
			models = mergeModels(models, pm)
		}
	}
	if err := models.Validate(); err != nil {
		return nil, fmt.Errorf("models: %w", err)
	}

	auth, err := LoadAuth(globalDir)
	if err != nil {
		return nil, err
	}

	return &Config{
		Settings:   settings,
		Models:     models.Models,
		GlobalDir:  globalDir,
		ProjectDir: projectDir,
		Auth:       auth,
		Workspace:  workspace,
	}, nil
}

func (c *Config) DefaultModelConfig() (ModelConfig, error) {
	name := c.Settings.DefaultModel
	provider := c.Settings.DefaultProvider
	if name != "" {
		for _, m := range c.Models {
			if m.Name == name {
				if provider == "" || m.Provider == provider {
					return m, nil
				}
			}
		}
	}
	if provider != "" {
		for _, m := range c.Models {
			if m.Provider == provider {
				return m, nil
			}
		}
	}
	if name == "" && len(c.Models) > 0 {
		name = c.Models[0].Name
	}
	for _, m := range c.Models {
		if m.Name == name {
			return m, nil
		}
	}
	if len(c.Models) == 0 {
		return ModelConfig{}, fmt.Errorf("no models configured in models.json")
	}
	if provider != "" {
		return ModelConfig{}, fmt.Errorf("no model for provider %q", provider)
	}
	return ModelConfig{}, fmt.Errorf("model %q not found", name)
}

func (c *Config) SessionsRoot() string {
	if dir := os.Getenv("STELL_SESSION_DIR"); dir != "" {
		return dir
	}
	if c.Settings.SessionDir != "" {
		return expandHome(c.Settings.SessionDir)
	}
	return filepath.Join(c.GlobalDir, "sessions")
}

func expandHome(p string) string {
	if p == "" {
		return p
	}
	if p[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[1:])
	}
	return p
}

func readSettings(path string) (Settings, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, false, nil
		}
		return Settings{}, false, err
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return Settings{}, false, fmt.Errorf("%s: %w", path, err)
	}
	s = applyProfileSettings(s, b)
	return s, true, nil
}

// applyProfileSettings сливает поля из profiles.<defaultProfile>, когда
// файл настроек верхнего уровня использует обёртку profile (externalEditor, theme и т.д.).
func applyProfileSettings(base Settings, raw []byte) Settings {
	var wrapper struct {
		DefaultProfile string                     `json:"defaultProfile"`
		Profiles       map[string]json.RawMessage `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil || len(wrapper.Profiles) == 0 {
		return base
	}
	name := wrapper.DefaultProfile
	if name == "" {
		name = "default"
	}
	profRaw, ok := wrapper.Profiles[name]
	if !ok {
		return base
	}
	var prof Settings
	if err := json.Unmarshal(profRaw, &prof); err != nil {
		return base
	}
	return mergeProfileFallback(base, prof)
}

func mergeProfileFallback(base, prof Settings) Settings {
	out := base
	if out.ExternalEditor == "" && prof.ExternalEditor != "" {
		out.ExternalEditor = prof.ExternalEditor
	}
	if out.MarkdownPager == "" && prof.MarkdownPager != "" {
		out.MarkdownPager = prof.MarkdownPager
	}
	if out.Theme == "" && prof.Theme != "" {
		out.Theme = prof.Theme
	}
	if out.DefaultModel == "" && prof.DefaultModel != "" {
		out.DefaultModel = prof.DefaultModel
	}
	if out.DefaultThinkingLevel == "" && prof.DefaultThinkingLevel != "" {
		out.DefaultThinkingLevel = prof.DefaultThinkingLevel
	}
	return out
}
