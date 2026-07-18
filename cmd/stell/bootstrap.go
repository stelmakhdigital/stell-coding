package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stelmakhdigital/stell-coding/internal/agent"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-coding/internal/discovery"
	"github.com/stelmakhdigital/stell-coding/internal/extensions"
	"github.com/stelmakhdigital/stell-agent/hooks"
	"github.com/stelmakhdigital/stell-coding/internal/packages"
	"github.com/stelmakhdigital/stell-ai/provider"
	"github.com/stelmakhdigital/stell-ai/provider/codex"
	"github.com/stelmakhdigital/stell-coding/internal/providerhooks"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-agent/tools"
	"github.com/stelmakhdigital/stell-coding/internal/trust"
)

type bootstrapOpts struct {
	continueSession bool
	resumeSession   bool
	sessionPath     string
	extensionDir    string
	apiKey          string
	autoApprove     bool
	noApprove       bool
	interactive     bool

	// Флаги инструментов (CLI).
	tools          string // список разрешённых через запятую
	excludeTools   string
	noTools        bool
	noBuiltinTools bool
	includeCoding  bool // включить grep/find/ls в набор по умолчанию
}

type App struct {
	Config      *config.Config
	Registry    *provider.Registry
	Tools       *tools.Runtime
	Sessions    *session.Manager
	SessPath    string
	Model       config.ModelConfig
	Catalog     *discovery.Catalog
	Extensions  *extensions.Supervisor
	GrantBroker *extensions.GrantBroker
	Service     *agent.Service
}

func bootstrap(workspaceFlag, modelName string, noSession bool, opts bootstrapOpts) (*App, error) {
	if noSession && opts.sessionPath != "" {
		return nil, fmt.Errorf("--session cannot be used with --no-session")
	}
	sessionPath := opts.sessionPath
	if sessionPath != "" {
		var err error
		sessionPath, err = filepath.Abs(sessionPath)
		if err != nil {
			return nil, err
		}
	}

	ws, err := resolveWorkspace(workspaceFlag, sessionPath)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(ws)
	if err != nil {
		return nil, err
	}
	if t := strings.TrimSpace(cfg.Settings.CodexTransport); t != "" {
		codex.SetDefaultTransport(t)
	}
	config.ApplyHTTPProxy(cfg.Settings.HTTPProxy)

	mc, err := cfg.DefaultModelConfig()
	if err != nil {
		return nil, err
	}
	if modelName != "" {
		found := false
		for _, m := range cfg.Models {
			if m.Name == modelName {
				mc = m
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("model %q not found", modelName)
		}
	}

	if opts.apiKey != "" && cfg.Auth != nil {
		cfg.Auth.SetCLIOverride(opts.apiKey)
	}

	reg, err := provider.BuildRegistry(cfg.Models, cfg.Auth)
	if err != nil {
		return nil, err
	}
	if opts.interactive && !config.HasAuthConfigured(cfg.Auth, mc) && !mc.Local {
		fmt.Fprintf(os.Stderr, "stell: model %q may need auth — run `stell login %s` or set API key\n", mc.Name, mc.Provider)
	}

	trusted, err := trust.EnsureTrust(
		cfg.GlobalDir, ws, cfg.Settings.DefaultProjectTrust,
		opts.autoApprove, opts.noApprove, opts.interactive,
	)
	if err != nil {
		return nil, err
	}
	tf, _ := trust.LoadMerged(cfg.GlobalDir, ws)
	if tf != nil && tf.IsTrusted(ws) {
		trusted = true
	}

	rt := tools.NewRuntime(tools.Env{
		Workspace:       ws,
		Trusted:         trusted,
		BashAutoApprove: opts.autoApprove || (tf != nil && tf.BashAutoApprove),
		BashDeny:        opts.noApprove,
	})
	if err := applyBuiltinTools(rt, opts); err != nil {
		return nil, err
	}

	sess := session.NewManager(ws)
	sessPath := ""
	if !noSession && sessionPath == "" {
		sessPath = session.NewSessionPath(cfg.SessionsRoot(), ws)
	}

	catalog, err := discovery.Load(cfg)
	if err != nil {
		return nil, err
	}

	if err := ensureConfiguredPackages(context.Background(), cfg); err != nil {
		return nil, err
	}

	grantBroker := extensions.NewGrantBroker()
	extSup, err := bootstrapExtensions(cfg, rt, ws, opts.extensionDir, grantBroker, opts.interactive)
	if err != nil {
		return nil, err
	}

	svc := agent.NewService(cfg, reg, rt, sess, sessPath, mc, catalog, extSup)
	svc.GrantBroker = grantBroker
	svc.InitProviderOverrides()
	extSup.SetHost(svc)
	wireHookBus(svc, extSup, rt, ws)
	reg.SetHTTPHooks(providerhooks.BusHTTPHooks(svc.Hooks))
	emitResourcesDiscover(svc, catalog)
	_ = svc.EmitSessionStart()
	svc.EmitProjectTrustHook(context.Background(), ws, trusted)

	if sessionPath != "" {
		if err := svc.OpenSession(sessionPath); err != nil {
			return nil, err
		}
		sessPath = sessionPath
		sess = svc.Sessions
	} else if opts.continueSession {
		if path, err := svc.ContinueSession(); err == nil {
			sessPath = path
			svc.SessPath = path
			sess = svc.Sessions
		}
	} else if opts.resumeSession {
		if path, err := svc.ResumeLatest(); err == nil {
			sessPath = path
			svc.SessPath = path
			sess = svc.Sessions
		}
	}

	return &App{
		Config:      cfg,
		Registry:    reg,
		Tools:       rt,
		Sessions:    sess,
		SessPath:    sessPath,
		Model:       mc,
		Catalog:     catalog,
		Extensions:  extSup,
		GrantBroker: grantBroker,
		Service:     svc,
	}, nil
}

// wireHookBus подключает адаптер subprocess-расширений и host-
// возможности (exec, sendUserMessage, appendEntry, UI input) к
// внутренней шине хуков.
func wireHookBus(svc *agent.Service, extSup *extensions.Supervisor, rt *tools.Runtime, ws string) {
	bus := svc.Hooks
	if extSup != nil {
		extSup.AttachBus(bus)
	}
	bus.SetHostCtx(&hooks.Ctx{
		Workspace: ws,
		ExecFn: func(ctx context.Context, command string) (hooks.ExecResult, error) {
			res, err := rt.RunBash(ctx, command)
			if err != nil {
				return hooks.ExecResult{}, err
			}
			return hooks.ExecResult{
				Output:    res.Output,
				ExitCode:  res.ExitCode,
				Truncated: res.Truncated,
				Cancelled: res.Cancelled,
			}, nil
		},
		SendUserMessageFn: svc.ExtensionSendUserMessage,
		AppendEntryFn:     svc.AppendCustomEntry,
		UIInputFn: func(ctx context.Context, message, placeholder, value string) (string, bool, error) {
			if extSup == nil || extSup.UIHost == nil {
				return "", false, fmt.Errorf("ui not configured")
			}
			return extSup.UIHost.Input(ctx, message, placeholder, value)
		},
	})
}

// emitResourcesDiscover вызывается один раз при старте после сборки
// каталога skills/prompts.
func emitResourcesDiscover(svc *agent.Service, catalog *discovery.Catalog) {
	payload := map[string]any{}
	if catalog != nil {
		if catalog.Skills != nil {
			payload["skills"] = len(catalog.Skills.List())
		}
		if catalog.Prompts != nil {
			payload["prompts"] = len(catalog.Prompts.List())
		}
	}
	_, _ = svc.Hooks.EmitNamed(context.Background(), hooks.ResourcesDiscover, svc.Sessions.Header.ID, payload)
}

// Shutdown эмитирует хук session_shutdown и останавливает subprocess-расширения.
func (a *App) Shutdown() {
	a.Service.EmitSessionShutdown(context.Background())
	if a.Extensions != nil {
		a.Extensions.Close()
	}
}

func absWorkspace(flag string) (string, error) {
	if flag == "" {
		return os.Getwd()
	}
	return filepath.Abs(flag)
}

func resolveWorkspace(workspaceFlag, sessionPath string) (string, error) {
	if workspaceFlag != "" {
		return absWorkspace(workspaceFlag)
	}
	if sessionPath != "" {
		sm, err := session.Open(sessionPath)
		if err != nil {
			return "", fmt.Errorf("session: %w", err)
		}
		if sm.Header.CWD != "" {
			return filepath.Abs(sm.Header.CWD)
		}
	}
	return os.Getwd()
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// applyBuiltinTools регистрирует встроенные инструменты и применяет --tools/--exclude-tools/--no-tools.
// Активный набор по умолчанию: read/write/edit/bash; grep/find/ls зарегистрированы, но неактивны.
func applyBuiltinTools(rt *tools.Runtime, opts bootstrapOpts) error {
	if opts.noBuiltinTools {
		if opts.noTools {
			rt.SetActiveTools([]string{})
		}
		return nil
	}
	if err := tools.RegisterBuiltins(rt); err != nil {
		return err
	}
	active, restrict := tools.ResolveActiveTools(tools.ToolSelection{
		Tools:         splitCSV(opts.tools),
		Exclude:       splitCSV(opts.excludeTools),
		NoTools:       opts.noTools,
		IncludeCoding: opts.includeCoding,
	}, rt.AllNames())
	if restrict {
		rt.SetActiveTools(active)
	}
	return nil
}

func ensureConfiguredPackages(ctx context.Context, cfg *config.Config) error {
	if len(cfg.Settings.Packages) == 0 {
		return nil
	}
	mgr := packages.NewManager(cfg.GlobalDir, cfg.ProjectDir, "project")
	installed := map[string]bool{}
	if recs, err := mgr.List(); err == nil {
		for _, r := range recs {
			installed[r.Name] = true
		}
	}
	for _, raw := range cfg.Settings.Packages {
		src, ok := raw.(string)
		if !ok || src == "" {
			continue
		}
		if recs, err := mgr.List(); err == nil {
			skip := false
			for _, r := range recs {
				if r.Source == src {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}
		rec, err := mgr.Install(ctx, src)
		if err != nil {
			return fmt.Errorf("settings.packages: %w", err)
		}
		if rec != nil {
			installed[rec.Name] = true
		}
	}
	_ = installed
	return nil
}
