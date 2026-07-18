package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/stelmakhdigital/stell-ai"
	"github.com/stelmakhdigital/stell-agent/hooks"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-coding/internal/packages"
	"github.com/stelmakhdigital/stell-agent/tools"
)

type CommandEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type PackageLister interface {
	List() ([]packages.Record, error)
}

type Supervisor struct {
	Workspace      string
	Packages       PackageLister
	Runtime        *tools.Runtime
	ExtraDirs      []string
	GrantBroker    *GrantBroker
	GrantChecker   GrantChecker
	Interactive    bool
	UI             *UIProtocol
	UIHost         *UIHost
	Host           HostBridge
	ExtensionError ExtensionErrorEmitter
	Renderers      *RendererRegistry
	CustomUI       *CustomUIRegistry
	CustomSessions *CustomSessionManager
	providers      *ProviderOverrides
	autocomplete   *autocompleteRegistry

	mu              sync.Mutex
	running         map[string]*runningExt
	statuses        []ReloadStatus
	shortcuts       []ShortcutDef
	flags           []FlagDef
	flagValues      map[string]any
	workflowMu      sync.Mutex
	workflows       map[string]context.CancelFunc
	workflowClients map[string]*ProcessClient
	WorkflowNotify  func(method string, params map[string]any)
}

type runningExt struct {
	pkgName    string
	extName    string
	dir        string
	manifest   *Manifest
	client     *ProcessClient
	toolNames  []string
	commands   []SlashCommandDef
	runningKey string
}

func NewSupervisor(workspace string, pkgs PackageLister, rt *tools.Runtime) *Supervisor {
	return &Supervisor{
		Workspace:       workspace,
		Packages:        pkgs,
		Runtime:         rt,
		Renderers:       NewRendererRegistry(),
		CustomUI:        NewCustomUIRegistry(),
		CustomSessions:  NewCustomSessionManager(),
		running:         map[string]*runningExt{},
		workflows:       map[string]context.CancelFunc{},
		workflowClients: map[string]*ProcessClient{},
	}
}

func (s *Supervisor) Bootstrap(ctx context.Context) error {
	s.mu.Lock()
	s.statuses = nil
	s.mu.Unlock()

	var statuses []ReloadStatus
	if s.Packages != nil {
		recs, err := s.Packages.List()
		if err != nil {
			return err
		}
		for _, rec := range recs {
			statuses = append(statuses, s.startPackage(ctx, rec)...)
		}
	}
	for _, dir := range s.ExtraDirs {
		statuses = append(statuses, s.startLooseDir(ctx, dir)...)
	}
	s.mu.Lock()
	s.statuses = statuses
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) Reload(ctx context.Context) ([]ReloadStatus, error) {
	s.stopAll()
	if err := s.Bootstrap(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ReloadStatus, len(s.statuses))
	copy(out, s.statuses)
	return out, nil
}

func (s *Supervisor) Close() {
	s.stopAll()
}

func (s *Supervisor) stopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.running {
		if s.Runtime != nil && len(e.toolNames) > 0 {
			s.Runtime.Unregister(e.toolNames...)
		}
		key := extensionKey(e)
		if s.providers != nil {
			s.providers.UnregisterOwner(key)
		}
		if s.autocomplete != nil {
			s.autocomplete.UnregisterOwner(key)
		}
		_ = e.client.Close()
	}
	s.running = map[string]*runningExt{}
	s.shortcuts = nil
	s.flags = nil
	s.flagValues = nil
}

func (s *Supervisor) SetProviderBaseModels(base []config.ModelConfig) {
	if s.providers == nil {
		s.providers = NewProviderOverrides(base)
		return
	}
	s.providers = NewProviderOverrides(base)
}

func (s *Supervisor) RegisterProvider(name string, cfg ProviderOverrideConfig, owner string) error {
	if s.providers == nil {
		s.providers = NewProviderOverrides(nil)
	}
	return s.providers.Register(name, cfg, owner)
}

func (s *Supervisor) UnregisterProvider(name string) {
	if s.providers != nil {
		s.providers.Unregister(name)
	}
}

func (s *Supervisor) EffectiveModels() []config.ModelConfig {
	if s.providers == nil {
		return nil
	}
	return s.providers.Models()
}

func (s *Supervisor) QueryAutocomplete(ctx context.Context, query string) []map[string]string {
	if s.autocomplete == nil {
		return nil
	}
	return s.autocomplete.Query(ctx, query)
}

func (s *Supervisor) startPackage(ctx context.Context, pkg packages.Record) []ReloadStatus {
	manifest, err := packages.LoadManifest(pkg.InstallPath)
	if err != nil {
		return []ReloadStatus{{Package: pkg.Name, OK: false, Error: err.Error()}}
	}
	dirs := manifest.ResourceDirs(pkg.InstallPath, "extensions")
	var out []ReloadStatus
	for _, dir := range dirs {
		for _, extDir := range listExtensionDirs(dir) {
			out = append(out, s.startExtensionDir(ctx, pkg.Name, extDir))
		}
	}
	return out
}

func listExtensionDirs(root string) []string {
	if _, err := LoadManifest(root); err == nil {
		return []string{root}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		d := filepath.Join(root, e.Name())
		if _, err := LoadManifest(d); err == nil {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

func (s *Supervisor) startLooseDir(ctx context.Context, dir string) []ReloadStatus {
	if _, err := LoadManifest(dir); err != nil {
		return nil
	}
	return []ReloadStatus{s.startExtensionDir(ctx, filepath.Base(dir), dir)}
}

func (s *Supervisor) startExtensionDir(ctx context.Context, pkgName, dir string) ReloadStatus {
	st := ReloadStatus{Package: pkgName}
	em, err := LoadManifest(dir)
	if err != nil {
		st.Error = err.Error()
		return st
	}
	if em.Type != TypeProcess {
		st.Extension = em.Name
		st.Error = fmt.Sprintf("unsupported extension type %q", em.Type)
		return st
	}
	st.Extension = em.Name

	if em.needsPermissions() && s.GrantChecker != nil && !s.GrantChecker.HasGrant(em.GrantKey(pkgName)) {
		ok, err := s.GrantChecker.RequestGrant(ctx, em.GrantKey(pkgName), em.Permissions)
		if err != nil || !ok {
			st.Error = fmt.Sprintf("extension %q permissions denied", em.Name)
			return st
		}
	}

	client, err := StartProcess(ctx, dir, em.Command)
	if err != nil {
		st.Error = err.Error()
		return st
	}
	client.Notify = func(method string, params map[string]any) {
		s.handleExtensionNotify(client, method, params)
	}
	client.HostRequest = func(method string, params json.RawMessage) (any, error) {
		owner := s.findOwner(client)
		return s.HandleHostRequest(ctx, client, owner, method, params)
	}
	initRes, err := client.Initialize(ctx, s.Workspace, pkgName)
	if err != nil {
		_ = client.Close()
		st.Error = err.Error()
		return st
	}

	toolNames := make([]string, 0, len(initRes.Tools))
	for _, td := range initRes.Tools {
		tool := &ProxyTool{client: client, def: td}
		if err := s.Runtime.Register(tool); err != nil {
			_ = client.Close()
			st.Error = fmt.Sprintf("tool conflict %q: %v", td.Name, err)
			return st
		}
		toolNames = append(toolNames, td.Name)
	}

	cmds := initRes.Commands
	if len(cmds) == 0 {
		cmds = manifestCommands(em)
	}
	key := pkgName + "/" + em.Name
	re := &runningExt{
		pkgName: pkgName, extName: em.Name, dir: dir, manifest: em,
		client: client, toolNames: toolNames, commands: cmds, runningKey: key,
	}
	for _, sc := range em.Shortcuts {
		s.registerShortcut(re, sc.Key, sc.Action)
	}
	for _, fl := range em.Flags {
		s.registerFlag(re, fl.Name, fl.Description, fl.Type, fl.Default)
	}
	s.mu.Lock()
	s.running[key] = re
	s.mu.Unlock()
	st.OK = true
	return st
}

func manifestCommands(m *Manifest) []SlashCommandDef {
	var out []SlashCommandDef
	for _, c := range m.Commands {
		name := normalizeSlash(c.Slash)
		if name == "" {
			continue
		}
		out = append(out, SlashCommandDef{Name: name, Description: c.Description})
	}
	return out
}

func normalizeSlash(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	return name
}

func (s *Supervisor) Commands() []CommandEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]bool{}
	var out []CommandEntry
	for _, e := range s.running {
		for _, c := range e.commands {
			name := normalizeSlash(c.Name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, CommandEntry{
				Name: name, Description: c.Description,
				Source: e.pkgName + "/" + e.extName,
			})
		}
	}
	return out
}

func (s *Supervisor) InvokeCommand(ctx context.Context, name string, args []string, sessionID string) (CommandResult, error) {
	name = normalizeSlash(name)
	e := s.findCommandOwner(name)
	if e == nil {
		return CommandResult{}, fmt.Errorf("unknown command %q", name)
	}
	return e.client.InvokeCommand(ctx, name, args, sessionID)
}

func (s *Supervisor) findCommandOwner(name string) *runningExt {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = normalizeSlash(name)
	for _, e := range s.running {
		for _, c := range e.commands {
			if normalizeSlash(c.Name) == name {
				return e
			}
		}
	}
	return nil
}

func (s *Supervisor) HasSubscriber(name string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.running {
		if e.manifest.Subscribes(name) {
			return true
		}
	}
	return false
}

func (s *Supervisor) EmitHook(ctx context.Context, name string, payload map[string]any) (map[string]any, error) {
	s.mu.Lock()
	exts := make([]*runningExt, 0, len(s.running))
	for _, e := range s.running {
		if e.manifest.Subscribes(name) {
			exts = append(exts, e)
		}
	}
	s.mu.Unlock()

	var merged map[string]any
	var firstErr error
	set := func(key string, val any) {
		if merged == nil {
			merged = map[string]any{}
		}
		merged[key] = val
	}
	for _, e := range exts {
		res, err := e.client.EmitHook(ctx, name, payload)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if res == nil {
			continue
		}
		// Общее слияние известных изменяемых ключей: appendSystem конкатенирует,
		// cancel/block через OR, args/text/command — last-writer-wins.
		if v, ok := res["appendSystem"].(string); ok && v != "" {
			var prev any
			if merged != nil {
				prev = merged["appendSystem"]
			}
			set("appendSystem", appendString(prev, v))
		}
		if cancel, _ := res["cancel"].(bool); cancel {
			set("cancel", true)
		}
		if block, _ := res["block"].(bool); block {
			set("block", true)
		}
		if args, ok := res["args"].(map[string]any); ok && len(args) > 0 {
			set("args", args)
		}
		if text, ok := res["text"].(string); ok && text != "" {
			set("text", text)
		}
		if cmd, ok := res["command"].(string); ok && strings.TrimSpace(cmd) != "" {
			set("command", cmd)
		}
	}
	if firstErr != nil {
		s.emitExtensionError("", firstErr.Error())
	}
	return merged, firstErr
}

// AttachBus подписывает subprocess-расширения на in-process шину хуков.
// Пробрасывает каждый эмитированный хук, на который подписано расширение
// (включая provider-хуки с serializable subset payload), и мержит ответы в Event.
func (s *Supervisor) AttachBus(bus *hooks.Bus) {
	if s == nil || bus == nil {
		return
	}
	bus.RegisterInterest(func(name string) bool {
		return s.HasSubscriber(name)
	})
	bus.OnAny(func(ctx context.Context, _ *hooks.Ctx, ev *hooks.Event) error {
		if !s.HasSubscriber(ev.Name) {
			return nil
		}
		// Provider-хуки: передаём сериализуемое подмножество (headers/status), чтобы
		// subprocess-расширения участвовали без полных потоков body.
		payload := map[string]any{}
		for k, v := range ev.Payload {
			payload[k] = v
		}
		if _, ok := payload["sessionId"]; !ok && ev.SessionID != "" {
			payload["sessionId"] = ev.SessionID
		}
		// Передаём текущие изменяемые значения, чтобы subprocess-расширения видели
		// перезаписи от более ранних in-process обработчиков.
		if ev.Args != nil {
			payload["args"] = ev.Args
		}
		if ev.Text != "" {
			payload["text"] = ev.Text
		}
		if ev.Command != "" {
			payload["command"] = ev.Command
		}
		merged, err := s.EmitHook(ctx, ev.Name, payload)
		applyHookResponse(ev, merged)
		return err
	})
}

// applyHookResponse сливает ответ subprocess-хука в изменяемые
// поля Event.
func applyHookResponse(ev *hooks.Event, merged map[string]any) {
	if merged == nil {
		return
	}
	if v, ok := merged["appendSystem"].(string); ok {
		ev.AppendSystemText(v)
	}
	if cancel, _ := merged["cancel"].(bool); cancel {
		ev.Cancel = true
	}
	if block, _ := merged["block"].(bool); block {
		ev.Block = true
	}
	if args, ok := merged["args"].(map[string]any); ok && len(args) > 0 {
		ev.Args = args
	}
	if text, ok := merged["text"].(string); ok && text != "" {
		ev.Text = text
	}
	if cmd, ok := merged["command"].(string); ok && cmd != "" {
		ev.Command = cmd
	}
}

func appendString(cur any, s string) string {
	if cur == nil {
		return s
	}
	if prev, ok := cur.(string); ok && prev != "" {
		return prev + "\n\n" + s
	}
	return s
}

type ProxyTool struct {
	client *ProcessClient
	def    ToolDef
}

func (t *ProxyTool) Def() ai.ToolDef {
	return ai.ToolDef{
		Name:        t.def.Name,
		Description: t.def.Description,
		Parameters:  t.def.Schema,
	}
}

func (t *ProxyTool) Call(ctx context.Context, env *tools.Env, args map[string]any) (tools.Result, error) {
	content, err := t.client.InvokeTool(ctx, t.def.Name, args)
	if err != nil {
		return tools.Result{}, err
	}
	return tools.Result{Content: content}, nil
}

func DiscoverLooseDirs(globalDir, projectDir string, extra []string) []string {
	var dirs []string
	add := func(root string) {
		if root == "" {
			return
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, filepath.Join(root, e.Name()))
			}
		}
	}
	add(filepath.Join(globalDir, "extensions"))
	if projectDir != "" {
		add(filepath.Join(projectDir, "extensions"))
	}
	for _, e := range extra {
		if info, err := os.Stat(e); err == nil && info.IsDir() {
			dirs = append(dirs, e)
		}
	}
	return dirs
}

func (s *Supervisor) RespondUI(id string, result map[string]any) {
	if s.UI != nil {
		s.UI.Respond(id, result)
	}
}

func (s *Supervisor) SetUIProtocol(ui *UIProtocol) {
	s.UI = ui
	s.UIHost = &UIHost{
		UI: ui,
		Themes: func() []map[string]string {
			if s.Host == nil {
				return nil
			}
			return s.Host.ExtensionListThemes()
		},
	}
}

func (s *Supervisor) RegisterWorkflow(runID string, cancel context.CancelFunc) {
	s.workflowMu.Lock()
	defer s.workflowMu.Unlock()
	if s.workflows == nil {
		s.workflows = map[string]context.CancelFunc{}
	}
	s.workflows[runID] = cancel
}

func (s *Supervisor) CancelWorkflow(runID string) error {
	s.workflowMu.Lock()
	cancel := s.workflows[runID]
	client := s.workflowClients[runID]
	delete(s.workflows, runID)
	delete(s.workflowClients, runID)
	s.workflowMu.Unlock()
	if cancel == nil && client == nil {
		return fmt.Errorf("workflow %q not found", runID)
	}
	if client != nil {
		_ = client.CancelWorkflow(context.Background(), runID)
	}
	if cancel != nil {
		cancel()
	}
	return nil
}

func (s *Supervisor) SetWorkflowNotify(fn func(method string, params map[string]any)) {
	s.WorkflowNotify = fn
}

func (s *Supervisor) handleExtensionNotify(client *ProcessClient, method string, params map[string]any) {
	switch method {
	case "workflow/register", "workflow/update", "workflow/step":
		if s.WorkflowNotify != nil {
			s.WorkflowNotify(method, params)
		}
		if method == "workflow/register" {
			runID, _ := params["runId"].(string)
			if runID != "" {
				_, cancel := context.WithCancel(context.Background())
				s.workflowMu.Lock()
				if s.workflows == nil {
					s.workflows = map[string]context.CancelFunc{}
				}
				if s.workflowClients == nil {
					s.workflowClients = map[string]*ProcessClient{}
				}
				s.workflows[runID] = cancel
				s.workflowClients[runID] = client
				s.workflowMu.Unlock()
			}
		}
		return
	case "ui/select", "ui/confirm", "ui/input", "ui/notify", "ui/setStatus", "ui/editor", "ui/setWidget", "ui/setTitle", "ui/setHeader",
		"ui/custom", "ui/setWorkingIndicator", "ui/setWorkingMessage", "ui/setWorkingVisible", "ui/replaceEditor":
		if s.UI == nil {
			return
		}
		id, _ := params["id"].(string)
		if id == "" {
			id = fmt.Sprintf("ext-ui-%d", time.Now().UnixNano())
		}
		kind := strings.TrimPrefix(method, "ui/")
		data := map[string]any{}
		for k, v := range params {
			if k != "id" {
				data[k] = v
			}
		}
		if kind == "custom" {
			mode, _ := data["mode"].(string)
			title, _ := data["title"].(string)
			lines := stringSliceAny(data["lines"])
			if content, _ := data["content"].(string); content != "" && len(lines) == 0 {
				lines = strings.Split(content, "\n")
			}
			owner := ""
			if client != nil {
				s.mu.Lock()
				for _, r := range s.running {
					if r.client == client {
						owner = r.runningKey
						break
					}
				}
				s.mu.Unlock()
			}
			if s.CustomSessions == nil {
				s.CustomSessions = NewCustomSessionManager()
			}
			sess := s.CustomSessions.Open(owner, mode, title, lines, data)
			if sess != nil {
				data["sessionId"] = sess.ID
				data["mode"] = sess.Mode
				data["lines"] = stringSliceToAny(sess.Lines)
				id = sess.ID
				if client != nil {
					_ = client.SendNotify("custom/opened", map[string]any{
						"id": sess.ID, "sessionId": sess.ID, "mode": sess.Mode,
					})
				}
			}
		}
		if kind == "select" || kind == "confirm" || kind == "input" || kind == "editor" || kind == "custom" {
			go func() {
				res := s.UI.Request(id, kind, data)
				_ = res
			}()
		} else {
			s.UI.Request(id, kind, data)
		}
	case "custom/paint":
		id, _ := params["id"].(string)
		if id == "" {
			id, _ = params["sessionId"].(string)
		}
		lines := stringSliceAny(params["lines"])
		if s.CustomSessions != nil && id != "" {
			s.CustomSessions.Paint(id, lines)
		}
		if s.UI != nil && id != "" {
			s.UI.Request(id, "customPaint", map[string]any{"sessionId": id, "lines": stringSliceToAny(lines)})
		}
	case "custom/done":
		id, _ := params["id"].(string)
		if id == "" {
			id, _ = params["sessionId"].(string)
		}
		result, _ := params["result"].(map[string]any)
		if s.CustomSessions != nil && id != "" {
			s.CustomSessions.Done(id, result)
		}
		if s.UI != nil && id != "" {
			s.UI.Respond(id, result)
		}
	}
}

func stringSliceAny(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		if ss, ok := v.([]string); ok {
			return ss
		}
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func stringSliceToAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// NotifyCustomKey пробрасывает key-событие расширению, владеющему custom session.
func (s *Supervisor) NotifyCustomKey(sessionID, key string) {
	if s == nil || s.CustomSessions == nil {
		return
	}
	sess := s.CustomSessions.Get(sessionID)
	if sess == nil || sess.Owner == "" {
		return
	}
	s.mu.Lock()
	var client *ProcessClient
	for _, r := range s.running {
		if r.runningKey == sess.Owner {
			client = r.client
			break
		}
	}
	s.mu.Unlock()
	if client == nil {
		return
	}
	_ = client.SendNotify("custom/key", map[string]any{"id": sessionID, "sessionId": sessionID, "key": key})
}
