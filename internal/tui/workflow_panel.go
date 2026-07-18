package tui

import (
	"fmt"
	"strings"
)

type workflowPanel struct {
	title   string
	steps   []string
	current int
	runID   string
}

func (m *Model) showWorkflowPanel(title string, steps []string, runID string) {
	m.workflow = &workflowPanel{
		title:   title,
		steps:   steps,
		current: 0,
		runID:   runID,
	}
}

func (m *Model) renderWorkflowPanel() string {
	if m.workflow == nil {
		return ""
	}
	w := m.workflow
	var b strings.Builder
	b.WriteString(m.colors.header().Render("workflow: " + w.title))
	b.WriteString("\n")
	for i, step := range w.steps {
		prefix := "  "
		if i == w.current {
			prefix = "> "
		}
		if i < w.current {
			prefix = "✓ "
		}
		b.WriteString(prefix)
		b.WriteString(step)
		b.WriteString("\n")
	}
	if w.runID != "" {
		b.WriteString(m.colors.muted().Render("run " + w.runID + " · esc cancel"))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) cancelWorkflow() {
	if m.workflow == nil {
		return
	}
	if m.workflow.runID != "" && m.svc.Extensions != nil {
		_ = m.svc.Extensions.CancelWorkflow(m.workflow.runID)
	}
	m.workflow = nil
	m.addInfo("workflow cancelled")
}

func (m *Model) advanceWorkflow(step int, title string) {
	if m.workflow == nil {
		m.showWorkflowPanel(title, nil, "")
	}
	if m.workflow != nil {
		m.workflow.current = step
		if title != "" {
			m.workflow.title = title
		}
	}
}

func workflowFromExtensionData(data map[string]any) (title string, steps []string, runID string) {
	title, _ = data["title"].(string)
	runID, _ = data["runId"].(string)
	if raw, ok := data["steps"].([]any); ok {
		for _, s := range raw {
			if str, ok := s.(string); ok {
				steps = append(steps, str)
			}
		}
	}
	if title == "" {
		title = "extension workflow"
	}
	return title, steps, runID
}

func (m *Model) handleWorkflowEvent(data map[string]any) {
	title, steps, runID := workflowFromExtensionData(data)
	if m.workflow == nil {
		m.showWorkflowPanel(title, steps, runID)
		return
	}
	if step, ok := data["step"].(float64); ok {
		m.advanceWorkflow(int(step), title)
	}
	if len(steps) > 0 {
		m.workflow.steps = steps
	}
	if runID != "" {
		m.workflow.runID = runID
	}
	m.addInfo(fmt.Sprintf("workflow: %s step %d", title, m.workflow.current))
}
