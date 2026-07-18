package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// HostBridge реализуется agent.Service для callbacks subprocess расширений.
type HostBridge interface {
	ExtensionSendUserMessage(ctx context.Context, message, deliverAs string) error
	ExtensionSendMessage(customType, text string, data json.RawMessage) (string, error)
	ExtensionReload(ctx context.Context) ([]ReloadStatus, error)
	ExtensionAppendEntry(text string) (string, error)
	ExtensionAppendTypedEntry(customType, text string, data json.RawMessage, asMessage bool) (string, error)
	ExtensionSetModel(name string) error
	ExtensionGetThinkingLevel() string
	ExtensionSetThinkingLevel(level string)
	ExtensionSetLabel(label string) error
	ExtensionRegisterProvider(name string, cfg ProviderOverrideConfig, owner string) error
	ExtensionUnregisterProvider(name string) error
	ExtensionListThemes() []map[string]string
}

// ExtensionErrorEmitter сообщает об ошибках расширений в потоки событий RPC/TUI.
type ExtensionErrorEmitter func(extension, message string)

// HandleHostRequest диспатчит методы host/* от subprocess расширения.
func (s *Supervisor) HandleHostRequest(ctx context.Context, client *ProcessClient, owner *runningExt, method string, params json.RawMessage) (any, error) {
	switch method {
	case "host/tools/register", "tools/register":
		var p struct {
			Tools []ToolDef `json:"tools"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if owner == nil {
			return nil, fmt.Errorf("unknown extension")
		}
		if err := s.registerTools(owner, p.Tools); err != nil {
			return nil, err
		}
		return map[string]any{"registered": len(p.Tools)}, nil
	case "host/tools/set_active", "tools/set_active":
		if s.Runtime == nil {
			return nil, fmt.Errorf("tool runtime not configured")
		}
		var p struct {
			Tools []string `json:"tools"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		s.Runtime.SetActiveTools(p.Tools)
		return map[string]any{"active": len(p.Tools)}, nil
	case "host/exec", "exec":
		if s.Runtime == nil {
			return nil, fmt.Errorf("tool runtime not configured")
		}
		var p struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		cmd := strings.TrimSpace(p.Command)
		if cmd == "" {
			return nil, fmt.Errorf("command required")
		}
		if owner != nil && s.GrantChecker != nil {
			key := extensionKey(owner)
			ok, err := s.GrantChecker.RequestGrant(ctx, key, ExtPermissions{Shell: true})
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("shell permission denied")
			}
		}
		res, err := s.Runtime.RunBash(ctx, cmd)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"output":   res.Output,
			"exitCode": res.ExitCode,
			"truncated": res.Truncated,
			"cancelled": res.Cancelled,
		}, nil
	case "host/agent/send_user_message", "host/agent/prompt", "agent/send_user_message", "agent/prompt":
		if s.Host == nil {
			return nil, fmt.Errorf("host bridge not configured")
		}
		var p struct {
			Message   string `json:"message"`
			DeliverAs string `json:"deliverAs"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.Message) == "" {
			return nil, fmt.Errorf("message required")
		}
		if err := s.Host.ExtensionSendUserMessage(ctx, p.Message, p.DeliverAs); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "host/agent/send_message", "agent/send_message":
		if s.Host == nil {
			return nil, fmt.Errorf("host bridge not configured")
		}
		var p struct {
			Message    string          `json:"message"`
			Text       string          `json:"text"`
			CustomType string          `json:"customType"`
			Data       json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		text := p.Message
		if text == "" {
			text = p.Text
		}
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("message required")
		}
		id, err := s.Host.ExtensionSendMessage(p.CustomType, text, p.Data)
		if err != nil {
			return nil, err
		}
		return map[string]any{"entryId": id}, nil
	case "host/agent/set_model", "agent/set_model":
		if s.Host == nil {
			return nil, fmt.Errorf("host bridge not configured")
		}
		var p struct {
			Model string `json:"model"`
			Name  string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		name := p.Model
		if name == "" {
			name = p.Name
		}
		if err := s.Host.ExtensionSetModel(name); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "model": name}, nil
	case "host/agent/get_thinking_level", "agent/get_thinking_level":
		if s.Host == nil {
			return nil, fmt.Errorf("host bridge not configured")
		}
		return map[string]any{"level": s.Host.ExtensionGetThinkingLevel()}, nil
	case "host/agent/set_thinking_level", "agent/set_thinking_level":
		if s.Host == nil {
			return nil, fmt.Errorf("host bridge not configured")
		}
		var p struct {
			Level string `json:"level"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		s.Host.ExtensionSetThinkingLevel(p.Level)
		return map[string]any{"ok": true, "level": p.Level}, nil
	case "host/agent/set_label", "agent/set_label":
		if s.Host == nil {
			return nil, fmt.Errorf("host bridge not configured")
		}
		var p struct {
			Label string `json:"label"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if err := s.Host.ExtensionSetLabel(p.Label); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "host/tools/get_all", "tools/get_all":
		if s.Runtime == nil {
			return map[string]any{"tools": []any{}}, nil
		}
		return map[string]any{"tools": s.Runtime.ListTools()}, nil
	case "host/commands/get", "commands/get":
		cmds := s.Commands()
		out := make([]map[string]string, 0, len(cmds))
		for _, c := range cmds {
			out = append(out, map[string]string{"name": c.Name, "description": c.Description})
		}
		return map[string]any{"commands": out}, nil
	case "host/reload", "reload":
		if s.Host == nil {
			st, err := s.Reload(ctx)
			return map[string]any{"extensions": st}, err
		}
		st, err := s.Host.ExtensionReload(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"extensions": st}, nil
	case "host/append_entry", "append_entry":
		if s.Host == nil {
			return nil, fmt.Errorf("host bridge not configured")
		}
		var p struct {
			Text       string          `json:"text"`
			CustomType string          `json:"customType"`
			Data       json.RawMessage `json:"data"`
			AsMessage  bool            `json:"asMessage"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		var id string
		var err error
		if p.CustomType != "" || len(p.Data) > 0 || p.AsMessage {
			id, err = s.Host.ExtensionAppendTypedEntry(p.CustomType, p.Text, p.Data, p.AsMessage)
		} else {
			id, err = s.Host.ExtensionAppendEntry(p.Text)
		}
		if err != nil {
			return nil, err
		}
		return map[string]any{"entryId": id}, nil
	case "host/ui/register_entry_renderer", "registerEntryRenderer":
		var p struct {
			CustomType string `json:"customType"`
			Label      string `json:"label"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if s.Renderers == nil {
			s.Renderers = NewRendererRegistry()
		}
		s.Renderers.RegisterEntryRenderer(p.CustomType, p.Label)
		return map[string]any{"ok": true}, nil
	case "host/ui/register_message_renderer", "registerMessageRenderer":
		var p struct {
			CustomType string `json:"customType"`
			Label      string `json:"label"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if s.Renderers == nil {
			s.Renderers = NewRendererRegistry()
		}
		s.Renderers.RegisterMessageRenderer(p.CustomType, p.Label)
		return map[string]any{"ok": true}, nil
	case "host/ui/register_custom", "ui/custom", "registerCustom":
		var p CustomUISlot
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if p.ID == "" {
			return nil, fmt.Errorf("id required")
		}
		if owner != nil {
			p.Owner = extensionKey(owner)
		}
		if s.CustomUI == nil {
			s.CustomUI = NewCustomUIRegistry()
		}
		s.CustomUI.Register(p)
		return map[string]any{"ok": true, "id": p.ID}, nil
	case "host/ui/input", "ui/input":
		if s.UIHost == nil {
			return nil, fmt.Errorf("ui not configured")
		}
		var p struct {
			Message     string `json:"message"`
			Placeholder string `json:"placeholder"`
			Value       string `json:"value"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		v, cancelled, err := s.UIHost.Input(ctx, p.Message, p.Placeholder, p.Value)
		if err != nil {
			return nil, err
		}
		return map[string]any{"value": v, "cancelled": cancelled}, nil
	case "host/register_shortcut", "registerShortcut":
		if owner == nil {
			return nil, fmt.Errorf("unknown extension")
		}
		var p struct {
			Key    string `json:"key"`
			Action string `json:"action"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if p.Key == "" || p.Action == "" {
			return nil, fmt.Errorf("key and action required")
		}
		s.registerShortcut(owner, p.Key, p.Action)
		return map[string]any{"ok": true}, nil
	case "host/register_flag", "registerFlag":
		if owner == nil {
			return nil, fmt.Errorf("unknown extension")
		}
		var p struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Type        string `json:"type"`
			Default     any    `json:"default"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if p.Name == "" {
			return nil, fmt.Errorf("name required")
		}
		s.registerFlag(owner, p.Name, p.Description, p.Type, p.Default)
		return map[string]any{"ok": true}, nil
	case "host/flags/get", "host/get_flag", "getFlag":
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if p.Name == "" {
			return nil, fmt.Errorf("name required")
		}
		v, ok := s.GetFlagValue(p.Name)
		if !ok {
			return nil, fmt.Errorf("flag %q not found", p.Name)
		}
		return map[string]any{"value": v}, nil
	case "host/providers/register", "host/register_provider", "registerProvider":
		if owner == nil {
			return nil, fmt.Errorf("unknown extension")
		}
		if s.Host == nil {
			return nil, fmt.Errorf("host bridge not configured")
		}
		var p struct {
			Name   string                 `json:"name"`
			Config ProviderOverrideConfig `json:"config"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		name := p.Name
		if name == "" {
			var flat struct {
				Name string `json:"provider"`
			}
			_ = json.Unmarshal(params, &flat)
			name = flat.Name
		}
		cfg := p.Config
		if cfg.BaseURL == "" {
			var flat ProviderOverrideConfig
			if json.Unmarshal(params, &flat) == nil {
				cfg = flat
			}
		}
		if name == "" {
			return nil, fmt.Errorf("provider name required")
		}
		if err := s.Host.ExtensionRegisterProvider(name, cfg, extensionKey(owner)); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "provider": name}, nil
	case "host/providers/unregister", "host/unregister_provider", "unregisterProvider":
		if s.Host == nil {
			return nil, fmt.Errorf("host bridge not configured")
		}
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if p.Name == "" {
			return nil, fmt.Errorf("name required")
		}
		if err := s.Host.ExtensionUnregisterProvider(p.Name); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "host/themes/list", "host/get_all_themes", "getAllThemes", "ui/themes/list", "host/ui/themes/list", "themes/list":
		if s.UIHost != nil {
			if themes := s.UIHost.ListThemes(); themes != nil {
				return map[string]any{"themes": themes}, nil
			}
		}
		if s.Host == nil {
			return map[string]any{"themes": []any{}}, nil
		}
		themes := s.Host.ExtensionListThemes()
		return map[string]any{"themes": themes}, nil
	case "host/ui/select", "ui/select":
		if s.UIHost == nil {
			return nil, fmt.Errorf("ui not configured")
		}
		var p struct {
			Message string   `json:"message"`
			Options []string `json:"options"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		v, cancelled, err := s.UIHost.Select(ctx, p.Message, p.Options)
		if err != nil {
			return nil, err
		}
		return map[string]any{"value": v, "cancelled": cancelled}, nil
	case "host/ui/confirm", "ui/confirm":
		if s.UIHost == nil {
			return nil, fmt.Errorf("ui not configured")
		}
		var p struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		ok, cancelled, err := s.UIHost.Confirm(ctx, p.Message)
		if err != nil {
			return nil, err
		}
		return map[string]any{"confirmed": ok, "cancelled": cancelled}, nil
	case "host/ui/set_header", "ui/setHeader", "setHeader":
		if s.UIHost == nil {
			return nil, fmt.Errorf("ui not configured")
		}
		var p struct {
			Lines   []string `json:"lines"`
			Content string   `json:"content"`
			Text    string   `json:"text"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		text := p.Content
		if text == "" {
			text = p.Text
		}
		if text == "" && len(p.Lines) > 0 {
			text = strings.Join(p.Lines, "\n")
		}
		s.UIHost.SetHeader(text)
		return map[string]any{"ok": true}, nil
	case "host/ui/register_autocomplete", "registerAutocompleteProvider":
		if owner == nil {
			return nil, fmt.Errorf("unknown extension")
		}
		var p struct {
			ID     string `json:"id"`
			Prefix string `json:"prefix"`
			Label  string `json:"label"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if p.ID == "" {
			p.ID = "ac-" + extensionKey(owner)
		}
		if s.autocomplete == nil {
			s.autocomplete = newAutocompleteRegistry()
		}
		s.autocomplete.Register(AutocompleteProvider{
			ID: p.ID, Prefix: p.Prefix, Label: p.Label,
			Owner: extensionKey(owner), Client: owner.client,
		})
		return map[string]any{"ok": true, "id": p.ID}, nil
	case "host/ui/set_working_indicator", "ui/setWorkingIndicator":
		if s.UIHost == nil {
			return nil, fmt.Errorf("ui not configured")
		}
		var p WorkingIndicatorOpts
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		s.UIHost.SetWorkingIndicator(p)
		return map[string]any{"ok": true}, nil
	case "host/ui/set_working_message", "ui/setWorkingMessage":
		if s.UIHost == nil {
			return nil, fmt.Errorf("ui not configured")
		}
		var p struct {
			Message string `json:"message"`
			Label   string `json:"label"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		msg := p.Message
		if msg == "" {
			msg = p.Label
		}
		s.UIHost.SetWorkingMessage(msg)
		return map[string]any{"ok": true}, nil
	case "host/ui/set_working_visible", "ui/setWorkingVisible":
		if s.UIHost == nil {
			return nil, fmt.Errorf("ui not configured")
		}
		var p struct {
			Visible *bool `json:"visible"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		s.UIHost.SetWorkingVisible(p.Visible)
		return map[string]any{"ok": true}, nil
	default:
		return nil, fmt.Errorf("unknown host method %q", method)
	}
}

func extensionKey(owner *runningExt) string {
	if owner == nil {
		return "extension"
	}
	if owner.pkgName != "" {
		return owner.pkgName + "/" + owner.extName
	}
	return owner.extName
}

func (s *Supervisor) registerShortcut(owner *runningExt, key, action string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := extensionKey(owner)
	s.shortcuts = append(s.shortcuts, ShortcutDef{Key: key, Action: action, Source: src})
}

func (s *Supervisor) registerFlag(owner *runningExt, name, desc, typ string, def any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := extensionKey(owner)
	if typ == "" {
		typ = "boolean"
	}
	s.flags = append(s.flags, FlagDef{Name: name, Description: desc, Type: typ, Default: def, Source: src})
	if s.flagValues == nil {
		s.flagValues = map[string]any{}
	}
	if _, ok := s.flagValues[name]; !ok && def != nil {
		s.flagValues[name] = def
	}
}

func (s *Supervisor) GetFlagValue(name string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.flagValues != nil {
		if v, ok := s.flagValues[name]; ok {
			return v, true
		}
	}
	for _, f := range s.flags {
		if f.Name == name {
			return f.Default, true
		}
	}
	return nil, false
}

func (s *Supervisor) registerTools(owner *runningExt, defs []ToolDef) error {
	if s.Runtime == nil {
		return fmt.Errorf("tool runtime not configured")
	}
	for _, td := range defs {
		tool := &ProxyTool{client: owner.client, def: td}
		if err := s.Runtime.RegisterOrReplace(tool); err != nil {
			return err
		}
		owner.toolNames = appendUnique(owner.toolNames, td.Name)
	}
	return nil
}

func appendUnique(list []string, name string) []string {
	for _, n := range list {
		if n == name {
			return list
		}
	}
	return append(list, name)
}

func (s *Supervisor) findOwner(client *ProcessClient) *runningExt {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.running {
		if e.client == client {
			return e
		}
	}
	return nil
}

func (s *Supervisor) SetHost(h HostBridge) {
	s.Host = h
}

func (s *Supervisor) SetExtensionErrorEmitter(fn ExtensionErrorEmitter) {
	s.ExtensionError = fn
}

func (s *Supervisor) emitExtensionError(ext, msg string) {
	if s.ExtensionError != nil {
		s.ExtensionError(ext, msg)
	}
}

// Shortcuts возвращает зарегистрированные shortcuts расширений.
func (s *Supervisor) Shortcuts() []ShortcutDef {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ShortcutDef, len(s.shortcuts))
	copy(out, s.shortcuts)
	return out
}

// Flags возвращает зарегистрированные flags расширений.
func (s *Supervisor) Flags() []FlagDef {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]FlagDef, len(s.flags))
	copy(out, s.flags)
	return out
}

// InvokeShortcut диспатчит действие shortcut во владеющее расширение.
func (s *Supervisor) InvokeShortcut(ctx context.Context, action string) error {
	s.mu.Lock()
	var src string
	for _, sc := range s.shortcuts {
		if sc.Action == action {
			src = sc.Source
			break
		}
	}
	var owner *runningExt
	if src != "" {
		for _, e := range s.running {
			if extensionKey(e) == src {
				owner = e
				break
			}
		}
	}
	s.mu.Unlock()
	if owner == nil {
		return fmt.Errorf("unknown shortcut action %q", action)
	}
	return owner.client.InvokeShortcut(ctx, action)
}
