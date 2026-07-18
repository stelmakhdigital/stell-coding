package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
)

func (m *Model) pasteFromClipboard() {
	if m.tryPasteClipboardImage() {
		return
	}
	text, err := clipboard.ReadAll()
	if err != nil || strings.TrimSpace(text) == "" {
		if m.errLine == "" {
			m.errLine = "Clipboard empty or no image — use ctrl+v / cmd+v"
		}
		return
	}
	m.errLine = ""
	text = strings.TrimSpace(text)
	if m.tryPasteImagePath(text) {
		return
	}
	cur := m.composer.Value()
	if cur != "" && !strings.HasSuffix(cur, " ") && !strings.HasSuffix(cur, "\n") {
		cur += " "
	}
	m.composer.SetValue(cur + text)
}

func (m *Model) tryPasteImagePath(text string) bool {
	path := text
	path = strings.TrimPrefix(path, "file://")
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
	default:
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.cfg.Workspace, path)
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return false
	}
	rel, err := filepath.Rel(m.cfg.Workspace, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel, err = m.importImageToWorkspace(path)
		if err != nil {
			m.errLine = "attach image: " + err.Error()
			return true
		}
		m.errLine = ""
		m.addAttachment(rel)
		return true
	}
	m.errLine = ""
	m.addAttachment(rel)
	return true
}

func (m *Model) importImageToWorkspace(absPath string) (string, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	name := ".stell-attachment-" + filepath.Base(absPath)
	dest := filepath.Join(m.cfg.Workspace, name)
	if err := os.WriteFile(dest, data, 0o600); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(m.cfg.Workspace, dest)
	if err != nil {
		return name, nil
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("image outside workspace")
	}
	return rel, nil
}

func (m *Model) copyLastAssistant() {
	text := m.svc.GetLastAssistantText()
	if text == "" {
		m.addInfo("no assistant message to copy")
		return
	}
	if err := clipboard.WriteAll(text); err != nil {
		m.addError("copy failed: " + err.Error())
		return
	}
	m.addInfo("copied assistant message")
}
