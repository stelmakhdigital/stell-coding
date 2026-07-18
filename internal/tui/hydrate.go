package tui

import (
	"fmt"
	"strings"

	"github.com/stelmakhdigital/stell-ai"
	"github.com/stelmakhdigital/stell-coding/internal/extensions"
	"github.com/stelmakhdigital/stell-agent/session"
)

func cardsFromSession(sess *session.Manager, renderers *extensions.RendererRegistry) []card {
	branch := sess.ActiveBranch()
	out := make([]card, 0, len(branch))
	for _, e := range branch {
		switch e.Type {
		case "message":
			if e.Message == nil {
				continue
			}
			if e.Message.Role == ai.RoleBashExecution {
				out = append(out, cardFromBashEntry(e))
				continue
			}
			if c, ok := cardFromMessage(*e.Message, e.CustomType, renderers); ok {
				out = append(out, c)
			}
		case "bash":
			if e.Message == nil {
				continue
			}
			out = append(out, cardFromBashEntry(e))
		case "custom", "custom_message", "session_info", "label":
			if e.Message == nil {
				continue
			}
			body := e.Message.Content
			if renderers != nil {
				var lines []string
				if e.Type == "custom_message" {
					lines = renderers.PaintMessage(e.CustomType, body, true)
				} else {
					lines = renderers.PaintEntry(e.CustomType, body, e.CustomData, true)
				}
				body = strings.Join(lines, "\n")
			} else if e.CustomType != "" {
				body = fmt.Sprintf("[%s] %s", e.CustomType, body)
			}
			out = append(out, card{kind: cardInfo, body: body, skillName: e.CustomType})
		case "compaction", "branch_summary", "model_change":
			if e.Message == nil {
				continue
			}
			label := e.Type
			if e.Type == "compaction" {
				label = "compacted"
			}
			out = append(out, card{kind: cardInfo, body: label + ": " + e.Message.Content})
		}
	}
	return out
}

func cardFromMessage(msg ai.Message, customType string, renderers *extensions.RendererRegistry) (card, bool) {
	ai.NormalizeMessage(&msg)
	if customType != "" && renderers != nil {
		return card{kind: cardInfo, body: renderers.FormatMessage(customType, msg.Content)}, true
	}
	switch msg.Role {
	case ai.RoleUser:
		c := cardFromUserContent(msg.Content)
		if len(msg.Images) > 0 {
			c.images = append([]ai.ImageContent(nil), msg.Images...)
		}
		return c, true
	case ai.RoleAssistant:
		body := msg.Content
		if body == "" {
			body = ai.ThinkingFromBlocks(msg.Blocks)
		}
		if body == "" && len(msg.ToolCalls) > 0 {
			return card{}, false
		}
		if body == "" && ai.HasAssistantOutput(msg.Blocks) {
			body = ai.ThinkingFromBlocks(msg.Blocks)
		}
		c := card{kind: cardAssistant, body: body}
		if len(msg.Images) > 0 {
			c.images = append([]ai.ImageContent(nil), msg.Images...)
		}
		return c, true
	case ai.RoleTool, ai.RoleToolLegacy:
		name := msg.ToolName
		if name == "" {
			name = "tool"
		}
		c := card{
			kind:        cardTool,
			body:        name + " → " + msg.Content,
			toolName:    name,
			toolContent: msg.Content,
			status:      cardStatusSuccess,
		}
		if len(msg.Images) > 0 {
			c.images = append([]ai.ImageContent(nil), msg.Images...)
		}
		return c, true
	default:
		if msg.Content == "" {
			return card{kind: cardInfo, body: string(msg.Role)}, true
		}
		return card{kind: cardInfo, body: string(msg.Role) + ": " + msg.Content}, true
	}
}

func cardFromBashEntry(e session.Entry) card {
	meta := session.EntryBashMeta(e)
	status := cardStatusSuccess
	if meta.Cancelled || meta.ExitCode != 0 {
		status = cardStatusError
	}
	content := ""
	cmd := ""
	if e.Message != nil {
		cmd = e.Message.Content
		if i := strings.Index(cmd, "\n"); i >= 0 {
			cmd = cmd[:i]
		}
		cmd = strings.TrimPrefix(strings.TrimPrefix(cmd, "$ !! "), "$ ")
		if parts := strings.SplitN(e.Message.Content, "\n", 2); len(parts) > 1 {
			content = parts[1]
		}
	}
	return card{
		kind:        cardBash,
		body:        formatBashEntryCard(e),
		toolName:    "bash",
		toolPath:    cmd,
		toolContent: content,
		status:      status,
		excludeBash: meta.ExcludeFromContext,
	}
}

func (m *Model) hydrateSession() {
	var renderers *extensions.RendererRegistry
	if m.svc.Extensions != nil {
		renderers = m.svc.Extensions.Renderers
	}
	m.lines = cardsFromSession(m.svc.Sessions, renderers)
	m.showStartup = len(m.lines) == 0
	m.syncViewport()
}
