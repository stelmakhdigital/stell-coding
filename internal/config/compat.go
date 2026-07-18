package config

import "github.com/stelmakhdigital/stell-ai"

// ResolveChatTemplateKwargs подставляет плейсхолдеры $var в chatTemplateKwargs.
func ResolveChatTemplateKwargs(raw map[string]any, level string, thinkMap map[string]*string) map[string]any {
	return ai.ResolveChatTemplateKwargs(raw, level, thinkMap)
}

// CloneCompatMap делает поверхностную копию JSON object map для merge.
func CloneCompatMap(m map[string]any) map[string]any {
	return ai.CloneCompatMap(m)
}
