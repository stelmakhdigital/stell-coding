package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (m *Model) openExternalEditor() {
	text := m.composer.Value()
	tmp, err := os.CreateTemp("", "stell-compose-*.md")
	if err != nil {
		m.addError(err.Error())
		return
	}
	path := tmp.Name()
	if _, err := tmp.WriteString(text); err != nil {
		_ = tmp.Close()
		m.addError(err.Error())
		return
	}
	_ = tmp.Close()

	editorCmd := m.cfg.Settings.ExternalEditorCommand()
	parts := strings.Fields(editorCmd)
	if len(parts) == 0 {
		m.addError("No editor configured. Set externalEditor in settings.json or $VISUAL/$EDITOR.")
		return
	}

	cmd := exec.CommandContext(m.ctx, parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			m.addInfo(fmt.Sprintf("'%s' exited with code %d. Keeping original text.", editorCmd, exitErr.ExitCode()))
		} else {
			m.addError(err.Error())
		}
		return
	}
	data, err := os.ReadFile(path)
	_ = os.Remove(path)
	if err != nil {
		m.addError(err.Error())
		return
	}
	m.composer.SetValue(string(data))
}

func (m *Model) toggleThinkingCollapsed() {
	m.thinkingCollapsed = !m.thinkingCollapsed
}
