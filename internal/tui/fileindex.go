package tui

import (

	"stell/coding-agent/internal/workspace"
)

type fileIndexMsg struct {
	files []string
	err   error
}

func (m *Model) indexWorkspaceFiles() Cmd {
	ws := m.cfg.Workspace
	return func() Msg {
		files, err := workspace.ListFiles(ws, 8000)
		return fileIndexMsg{files: files, err: err}
	}
}

func (m *Model) filterFiles(query string) []string {
	if len(m.fileIndex) > 0 {
		return workspace.FuzzyFilter(m.fileIndex, query, 40)
	}
	files, err := listWorkspaceFiles(m.cfg.Workspace, query)
	if err != nil {
		return nil
	}
	return files
}

func (m *Model) composerPopup() string {
	if m.overlayMode != overlayInlineAt || len(m.pickerFiles) == 0 {
		return ""
	}
	return renderScrollPopupStyled(
		"@ picker (↑/↓ tab enter, esc)",
		m.pickerFiles,
		m.overlayCursor,
		popupMaxRows,
		colorSeq(m.colors.Muted),
		colorSeq(m.colors.Accent),
	)
}
