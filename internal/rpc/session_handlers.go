package rpc

import (
	"context"
	"encoding/json"

	"stell/coding-agent/internal/agent"
)

func (s *Server) handleSessionCommands(ctx context.Context, id, typ string, raw json.RawMessage) bool {
	switch typ {
	case "get_entries":
		var p struct {
			Since  string `json:"since"`
			LeafID string `json:"leafId"`
		}
		_ = json.Unmarshal(raw, &p)
		prevLeaf := s.Svc.Sessions.LeafID()
		if p.LeafID != "" {
			_ = s.Svc.Sessions.SetLeaf(p.LeafID)
		}
		entries := s.Svc.Sessions.FilterEntriesSince(p.Since)
		leafID := s.Svc.Sessions.LeafID()
		if p.LeafID != "" && prevLeaf != "" {
			_ = s.Svc.Sessions.SetLeaf(prevLeaf)
		}
		s.respond(id, typ, true, map[string]any{"entries": entries, "leafId": leafID}, "")
		return true
	case "get_tree":
		s.Svc.EmitBeforeTree(ctx)
		nested := s.Svc.Sessions.BuildNestedTree()
		s.Svc.EmitSessionTree(ctx, len(s.Svc.Sessions.Entries))
		s.respond(id, typ, true, map[string]any{"tree": nested, "leafId": s.Svc.Sessions.LeafID()}, "")
		return true
	case "open_session":
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
		if err := s.Svc.OpenSession(path); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{
			"sessionId": s.Svc.Sessions.Header.ID,
			"path":      s.Svc.SessPath,
		}, "")
		return true
	case "fork":
		var p struct {
			EntryID string `json:"entryId"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		leaf, err := s.Svc.ForkSession(p.EntryID)
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{"leafId": leaf}, "")
		return true
	case "clone":
		path, err := s.Svc.CloneSession()
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{
			"path":      path,
			"sessionId": s.Svc.Sessions.Header.ID,
		}, "")
		return true
	case "compact":
		emitCompactionEvents(s, true, "manual")
		var p struct {
			CustomInstructions string `json:"customInstructions"`
		}
		_ = json.Unmarshal(raw, &p)
		info, err := s.Svc.CompactWithInstructions(ctx, p.CustomInstructions)
		if err != nil {
			emitCompactionEvents(s, false, map[string]any{"error": err.Error()})
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		emitCompactionEvents(s, false, info)
		s.respond(id, typ, true, map[string]any{"compact": info}, "")
		return true
	case "switch_to_entry":
		var p struct {
			EntryID string `json:"entryId"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		if err := s.Svc.SwitchToEntry(ctx, p.EntryID); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{"leafId": s.Svc.Sessions.LeafID()}, "")
		return true
	case "bash":
		var p struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		res, err := s.Svc.RunBash(ctx, p.Command, agent.RunBashOptions{})
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{
			"output":         res.Output,
			"exitCode":       res.ExitCode,
			"truncated":      res.Truncated,
			"cancelled":      res.Cancelled,
			"fullOutputPath": res.FullOutputPath,
		}, "")
		return true
	case "cycle_model":
		name, err := s.Svc.CycleModel()
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{"modelName": name}, "")
		return true
	case "list_sessions":
		files, err := s.Svc.ListSessions()
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{"sessions": files}, "")
		return true
	case "append_session_info":
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		entryID, err := s.Svc.AppendSessionInfo(p.Text)
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{"entryId": entryID}, "")
		return true
	case "append_custom_entry":
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		entryID, err := s.Svc.AppendCustomEntry(p.Text)
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{"entryId": entryID}, "")
		return true
	case "append_custom_message":
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		entryID, err := s.Svc.AppendCustomMessage(p.Text)
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{"entryId": entryID}, "")
		return true
	case "append_label":
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		entryID, err := s.Svc.AppendLabel(p.Text)
		if err != nil {
			s.respond(id, typ, false, nil, err.Error())
			return true
		}
		s.respond(id, typ, true, map[string]any{"entryId": entryID}, "")
		return true
	default:
		return false
	}
}
