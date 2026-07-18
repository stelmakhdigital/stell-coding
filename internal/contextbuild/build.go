package contextbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stelmakhdigital/ai"
)

type Options struct {
	SkillsPrompt string
	AppendSystem string
	// Tools — активные определения инструментов. PromptSnippet и
	// PromptGuidelines заполняют секции "Available tools" и "Guidelines"
	// дефолтного промпта (игнорируются, если SYSTEM.md переопределяет промпт).
	Tools []ai.ToolDef
}

// BuildSystemLegacy — алиас без skills (для тестов).
func BuildSystemLegacy(globalDir, workspace string) string {
	return BuildSystem(globalDir, workspace, Options{})
}

func BuildSystem(globalDir, workspace string, opts Options) string {
	var parts []string

	if p := findFile(workspace, "SYSTEM.md"); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			parts = append(parts, strings.TrimSpace(string(b)))
		}
	} else {
		parts = append(parts, defaultSystemPrompt(opts.Tools))
	}

	if p := findFile(workspace, "APPEND_SYSTEM.md"); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			parts = append(parts, strings.TrimSpace(string(b)))
		}
	}

	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if p := findFile(workspace, name); p != "" {
			if b, err := os.ReadFile(p); err == nil {
				parts = append(parts, strings.TrimSpace(string(b)))
			}
			break
		}
	}

	parts = append(parts, LoadContextSlots(globalDir, workspace)...)

	if opts.SkillsPrompt != "" {
		parts = append(parts, strings.TrimPrefix(opts.SkillsPrompt, "\n\n"))
	}

	if opts.AppendSystem != "" {
		parts = append(parts, opts.AppendSystem)
	}

	// Дата и рабочая директория — в конце, даже при кастомном SYSTEM.md,
	// чтобы модель не угадывала окружение.
	parts = append(parts, environmentFooter(workspace))

	return strings.Join(parts, "\n\n")
}

// defaultSystemPrompt собирает секции identity, списка инструментов и guidelines
// из активных инструментов (структура core-промпта по умолчанию).
func defaultSystemPrompt(tools []ai.ToolDef) string {
	var toolLines []string
	var guidelines []string
	seen := map[string]bool{}
	addGuideline := func(g string) {
		g = strings.TrimSpace(g)
		if g == "" || seen[g] {
			return
		}
		seen[g] = true
		guidelines = append(guidelines, g)
	}

	has := map[string]bool{}
	for _, t := range tools {
		has[t.Name] = true
		if s := oneLine(t.PromptSnippet); s != "" {
			toolLines = append(toolLines, fmt.Sprintf("- %s: %s", t.Name, s))
		}
		for _, g := range t.PromptGuidelines {
			addGuideline(g)
		}
	}

	if has["bash"] && !has["grep"] && !has["find"] && !has["ls"] {
		addGuideline("Use bash for file operations like ls, rg, find")
	}
	addGuideline("Be concise in your responses")
	addGuideline("Show file paths clearly when working with files")

	toolsList := "(none)"
	if len(toolLines) > 0 {
		toolsList = strings.Join(toolLines, "\n")
	}
	guidelinesList := make([]string, 0, len(guidelines))
	for _, g := range guidelines {
		guidelinesList = append(guidelinesList, "- "+g)
	}

	return fmt.Sprintf(`You are stell, a minimal coding agent. You help users by reading files, executing commands, editing code, and writing new files.

Available tools:
%s

In addition to the tools above, you may have access to other custom tools depending on the project.

Guidelines:
%s`, toolsList, strings.Join(guidelinesList, "\n"))
}

func environmentFooter(workspace string) string {
	cwd := filepath.ToSlash(workspace)
	date := time.Now().Format("2006-01-02")
	return fmt.Sprintf("Current date: %s\nCurrent working directory: %s", date, cwd)
}

func oneLine(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func findFile(start, name string) string {
	dir := filepath.Clean(start)
	for {
		p := filepath.Join(dir, name)
		if fileExists(p) {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
