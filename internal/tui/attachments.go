package tui

import "stell/coding-agent/internal/agent"

func (m *Model) prepareAttachments() error {
	imgPaths, filePaths := agent.SplitAttachments(m.attachmentPaths())
	if len(imgPaths) > 0 && !agent.ModelSupportsImages(m.svc.ActiveModelConfig()) {
		m.addInfo("model does not support images; using text placeholders only")
	}
	m.svc.SetPendingAttachments(filePaths)
	if len(imgPaths) == 0 {
		m.svc.SetPendingImages(nil)
		return nil
	}
	imgs, err := agent.LoadImages(m.cfg.Workspace, imgPaths)
	if err != nil {
		m.svc.SetPendingAttachments(nil)
		m.svc.SetPendingImages(nil)
		return err
	}
	m.svc.SetPendingImages(imgs)
	return nil
}

func (m *Model) clearSubmitComposer() {
	m.clearComposer()
	if m.slashMenu != nil {
		m.slashMenu = nil
		m.resizeViewport()
	}
}
