package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strings"

	"stell/coding-agent/internal/agent"
	"github.com/stelmakhdigital/ai"
	"stell/coding-agent/internal/extensions"
	"stell/agent/session"
	"stell/agent/tools"
)

func (s *Server) handleExtendedCommands(ctx context.Context, id, typ string, raw json.RawMessage) bool {
	switch typ {
	case "get_thinking_level":
		s.respond(id, typ, true, map[string]any{"thinkingLevel": s.Svc.GetThinkingLevel()}, "")
		return true
	case "set_thinking_level":
		var p struct {
			Level string `json:"level"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.Svc.SetThinkingLevel(p.Level)
		s.respond(id, typ, true, nil, "")
		return true
	case "cycle_thinking_level":
		level := s.Svc.CycleThinkingLevel()
		s.respond(id, typ, true, map[string]any{"thinkingLevel": level}, "")
		return true
	case "extension_grant":
		var p struct {
			ID      string `json:"id"`
			Granted bool   `json:"granted"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		if s.GrantBroker == nil {
			s.respond(id, typ, false, nil, "grant broker not configured")
			return true
		}
		if err := s.GrantBroker.Respond(p.ID, p.Granted); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, nil, "")
		return true
	case "extension_ui_response":
		var p struct {
			ID     string         `json:"id"`
			Result map[string]any `json:"result"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		if s.Svc.Extensions != nil {
			s.Svc.Extensions.RespondUI(p.ID, p.Result)
		}
		s.respond(id, typ, true, nil, "")
		return true
	case "set_auto_compaction":
		var p struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.Svc.SetAutoCompaction(p.Enabled)
		s.respond(id, typ, true, nil, "")
		return true
	case "set_auto_retry":
		var p struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.Svc.SetAutoRetry(p.Enabled)
		s.respond(id, typ, true, nil, "")
		return true
	case "abort_retry":
		s.Svc.AbortRetry()
		s.respond(id, typ, true, nil, "")
		return true
	case "abort_bash":
		s.Svc.AbortBash()
		s.respond(id, typ, true, nil, "")
		return true
	case "get_session_stats":
		s.respond(id, typ, true, s.Svc.GetSessionStats(), "")
		return true
	case "export_html":
		var p struct {
			Path       string `json:"path"`
			OutputPath string `json:"outputPath"`
		}
		_ = json.Unmarshal(raw, &p)
		dest := p.OutputPath
		if dest == "" {
			dest = p.Path
		}
		if dest == "" {
			dest = "session-export.html"
		}
		abs, _, err := tools.ResolveOutputPath(s.Svc.Config.Workspace, dest, "session-export.html")
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		if err := exportSessionHTML(s.Svc, abs); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{"path": abs}, "")
		return true
	case "get_fork_messages":
		var p struct {
			EntryID string `json:"entryId"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		msgs, err := s.Svc.GetForkMessages(p.EntryID)
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{"messages": msgs}, "")
		return true
	case "get_last_assistant_text":
		s.respond(id, typ, true, map[string]any{"text": s.Svc.GetLastAssistantText()}, "")
		return true
	case "set_session_name":
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.Svc.SetSessionName(p.Name)
		s.respond(id, typ, true, nil, "")
		return true
	case "switch_session":
		var p struct {
			Path        string `json:"path"`
			SessionPath string `json:"sessionPath"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		path := p.SessionPath
		if path == "" {
			path = p.Path
		}
		cancelled, err := s.Svc.SwitchSession(ctx, path)
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{
			"cancelled": cancelled,
			"sessionId": s.Svc.Sessions.Header.ID,
			"path":      s.Svc.SessPath,
		}, "")
		return true
	default:
		return false
	}
}

func (s *Server) WireCompactionEmitter() {
	s.Svc.CompactionEmitter = func(start bool, reason string, info any) {
		if start {
			emitCompactionEvents(s, true, reason)
		} else {
			emitCompactionEvents(s, false, info)
		}
	}
}

func (s *Server) WireUIProtocol(ext *extensions.Supervisor) {
	if ext == nil {
		return
	}
	ui := extensions.NewUIProtocol(func(req extensions.UIRequest) {
		s.emit(map[string]any{
			"type":   "extension_ui_request",
			"id":     req.ID,
			"method": req.Kind,
			"data":   req.Data,
		})
	})
	ext.SetUIProtocol(ui)
	ext.SetHost(s.Svc)
	ext.SetExtensionErrorEmitter(func(extension, message string) {
		s.emit(map[string]any{
			"type":      "extension_error",
			"extension": extension,
			"message":   message,
		})
	})
}

func (s *Server) WireGrantBroker(b *extensions.GrantBroker) {
	s.GrantBroker = b
	if b == nil {
		return
	}
	b.SetEmitter(func(req extensions.GrantRequest) {
		s.emit(map[string]any{
			"type":  "extension_grant_request",
			"id":    req.ID,
			"key":   req.Key,
			"perms": req.Perms,
		})
	})
}

func exportSessionHTML(svc *agent.Service, dest string) error {
	branch := svc.Sessions.ActiveBranch()
	stats := svc.GetSessionStats()
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>stell session</title>")
	b.WriteString("<style>")
	b.WriteString("body{font-family:system-ui,sans-serif;margin:0;display:flex;min-height:100vh}")
	b.WriteString(".sidebar{width:260px;background:#f5f5f5;border-right:1px solid #ddd;padding:1rem;font-size:.85em;overflow:auto}")
	b.WriteString(".main{flex:1;padding:1.5rem;max-width:900px}")
	b.WriteString(".entry{margin:1rem 0;padding:1rem;border-radius:8px;border:1px solid #ddd}")
	b.WriteString(".user{background:#f0f7ff}.assistant{background:#f8f8f8}.tool{background:#fff8e6}")
	b.WriteString(".info{background:#fafafa;color:#555;font-size:.9em}.thinking{color:#666;font-style:italic}")
	b.WriteString(".toolcard{font-family:monospace;font-size:.85em;margin:.5rem 0;padding:.5rem;background:#fff;border:1px solid #eee}")
	b.WriteString(".tree ul{list-style:none;padding-left:1rem;margin:.25rem 0}")
	b.WriteString(".tree li{margin:.15rem 0}.leaf{font-weight:600}")
	b.WriteString(".meta{color:#666;font-size:.9em;margin-bottom:1rem}")
	b.WriteString("</style></head><body>")
	b.WriteString(`<aside class="sidebar"><h2>Tree</h2>`)
	writeTreeHTML(&b, svc.Sessions.BuildNestedTree(), svc.Sessions.LeafID())
	b.WriteString("</aside><main class=\"main\">")
	b.WriteString("<h1>Session export</h1>")
	writeExportMeta(&b, stats)
	for _, e := range branch {
		writeEntryHTML(&b, e)
	}
	b.WriteString("</main></body></html>")
	return os.WriteFile(dest, []byte(b.String()), 0o644)
}

func writeExportMeta(b *strings.Builder, stats map[string]any) {
	b.WriteString(`<div class="meta">`)
	fmt.Fprintf(b, "Session: %s<br>", html.EscapeString(fmt.Sprint(stats["sessionId"])))
	fmt.Fprintf(b, "Model: %s (%s)<br>", html.EscapeString(fmt.Sprint(stats["modelName"])), html.EscapeString(fmt.Sprint(stats["provider"])))
	if cost, ok := stats["cost"]; ok {
		fmt.Fprintf(b, "Cost: %v<br>", cost)
	}
	if tokens, ok := stats["tokens"].(map[string]any); ok {
		fmt.Fprintf(b, "Tokens: in=%v out=%v<br>", tokens["input"], tokens["output"])
	}
	b.WriteString("</div>")
}

func writeTreeHTML(b *strings.Builder, nodes []session.NestedTreeNode, leafID string) {
	b.WriteString(`<div class="tree"><ul>`)
	for _, n := range nodes {
		writeTreeNodeHTML(b, n, leafID)
	}
	b.WriteString("</ul></div>")
}

func writeTreeNodeHTML(b *strings.Builder, n session.NestedTreeNode, leafID string) {
	cls := ""
	if n.Entry.ID == leafID {
		cls = ` class="leaf"`
	}
	label := n.Entry.Type
	if n.Entry.Label != "" {
		label = n.Entry.Label
	}
	fmt.Fprintf(b, `<li%s>%s <small>(%s)</small>`, cls, html.EscapeString(label), html.EscapeString(n.Entry.ID[:min(8, len(n.Entry.ID))]))
	if len(n.Children) > 0 {
		b.WriteString("<ul>")
		for _, c := range n.Children {
			writeTreeNodeHTML(b, c, leafID)
		}
		b.WriteString("</ul>")
	}
	b.WriteString("</li>")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func writeEntryHTML(b *strings.Builder, e session.Entry) {
	if e.Message == nil && e.Type != "label" {
		return
	}
	cls := "info"
	role := e.Type
	content := ""
	if e.Message != nil {
		role = string(e.Message.Role)
		content = e.Message.Content
	}
	switch e.Type {
	case "message":
		switch e.Message.Role {
		case ai.RoleUser:
			cls = "user"
		case ai.RoleAssistant:
			cls = "assistant"
		case ai.RoleTool, ai.RoleToolLegacy:
			cls = "tool"
		}
	case "bash":
		cls = "tool"
		role = "bash"
	case "custom", "custom_message", "session_info", "label", "compaction", "branch_summary", "model_change":
		cls = "info"
		role = e.Type
		if e.CustomType != "" {
			role = e.CustomType
		}
	}
	fmt.Fprintf(b, `<div class="entry %s"><strong>%s</strong>`, cls, html.EscapeString(role))
	if e.Timestamp != "" {
		fmt.Fprintf(b, ` <small>%s</small>`, html.EscapeString(e.Timestamp))
	}
	b.WriteString("<div>")
	if content != "" {
		fmt.Fprintf(b, "<pre>%s</pre>", html.EscapeString(content))
	}
	if e.Message != nil && len(e.Message.ToolCalls) > 0 {
		for _, tc := range e.Message.ToolCalls {
			args, _ := json.Marshal(tc.Args)
			fmt.Fprintf(b, `<div class="toolcard">⚙ %s %s</div>`, html.EscapeString(tc.Name), html.EscapeString(string(args)))
		}
	}
	b.WriteString("</div></div>\n")
}

func emitCompactionEvents(s *Server, start bool, info any) {
	EmitCompactionEvent(s.emit, start, info)
}

func EmitCompactionEvent(emit func(map[string]any), start bool, info any) {
	if start {
		reason := "manual"
		if r, ok := info.(string); ok && r != "" {
			reason = r
		}
		emit(map[string]any{"type": "compaction_start", "reason": reason})
	} else {
		emit(map[string]any{"type": "compaction_end", "info": info})
	}
}

func emitTurnEvents(s *Server, start bool) {
	emitTurnEvent(s.emit, start)
}

func emitTurnEvent(emit func(map[string]any), start bool) {
	if start {
		emit(map[string]any{"type": "turn_start"})
	} else {
		emit(map[string]any{"type": "turn_end"})
	}
}

// StreamAgentEvents пишет stell JSONL-события из канала событий агента.
func StreamAgentEvents(emit func(map[string]any), events <-chan agent.Event) int {
	exitCode := 0
	emit(map[string]any{"type": "agent_start"})
	emitTurnEvent(emit, true)
	mapper := NewEventMapper()
	for ev := range events {
		if ev.Type == agent.EventDone {
			for _, obj := range mapper.Map(ev) {
				emit(obj)
			}
			emitTurnEvent(emit, false)
			continue
		}
		if ev.Type == agent.EventError && ev.Err != nil {
			exitCode = 1
		}
		for _, obj := range mapper.Map(ev) {
			emit(obj)
		}
	}
	emit(map[string]any{"type": "agent_settled"})
	return exitCode
}

