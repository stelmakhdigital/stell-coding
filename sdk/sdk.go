// Package sdk — встраиваемый Go API для stell coding-agent.
package sdk

import (
	"context"
	"fmt"

	"github.com/stelmakhdigital/ai"
	"github.com/stelmakhdigital/ai/provider"
	_ "github.com/stelmakhdigital/ai/provider/all"
	coreagent "stell/agent"
	"stell/agent/session"
	"stell/agent/tools"
	"stell/coding-agent/internal/agent"
	"stell/coding-agent/internal/config"
	"stell/coding-agent/internal/discovery"
)

// Options настраивает CreateSession.
type Options struct {
	Workspace string
	AgentDir  string // необязательный override глобального каталога конфига
	Model     string
	// ThinkingLevel задаёт уровень reasoning по умолчанию для сессии.
	ThinkingLevel string
	// ScopedModels — имена моделей, доступные для CycleModel / переключения.
	ScopedModels []string
	// NoTools: "all" отключает все инструменты; "builtin" пропускает регистрацию встроенных.
	// Пусто при незаданном Tools → builtins по умолчанию (+ coding при IncludeCoding).
	NoTools string
	// Tools — allowlist имён инструментов (если задан, активны только они).
	Tools []string
	// ExcludeTools удаляет имена после Tools / defaults.
	ExcludeTools []string
	// IncludeCoding включает grep/find/ls при использовании builtins по умолчанию.
	IncludeCoding bool
	// CustomTools регистрируются после builtins.
	CustomTools []tools.Tool
	// Auth подставляет предзагруженный auth store (опционально).
	Auth *ai.Auth
	// Registry подставляет registry провайдеров (опционально; иначе собирается из models).
	Registry *provider.Registry
	// ResourceLoader подставляет discovery (опционально).
	ResourceLoader ResourceLoader
	// Continue возобновляет последнюю сессию workspace.
	Continue bool
	// Resume загружает SessionPath, если задан.
	Resume bool
	// SessionPath — явный путь к JSONL (с Resume или отдельно).
	SessionPath string
	// StreamFn переопределяет транспорт LLM Chat (например stell/agent/proxy.StreamProxy).
	// При nil учитываются STELL_PROXY_URL / STELL_PROXY_TOKEN через Service.
	StreamFn coreagent.StreamFn
}

// Session — in-process сессия агента (без TUI / RPC-транспорта).
type Session struct {
	Service        *agent.Service
	Config         *config.Config
	Registry       *provider.Registry
	SessPath       string
	ResourceLoader ResourceLoader
	Diagnostics    []ResourceDiagnostic
}

// CreateSession поднимает config, tools, registry и новую JSONL-сессию для workspace.
func CreateSession(workspace string) (*Session, error) {
	return CreateSessionOpts(Options{Workspace: workspace})
}

// CreateSessionOpts — полный конструктор сессии.
func CreateSessionOpts(opts Options) (*Session, error) {
	ws := opts.Workspace
	if ws == "" {
		ws = "."
	}
	cfg, err := config.Load(ws)
	if err != nil {
		return nil, err
	}
	if opts.AgentDir != "" {
		cfg.GlobalDir = opts.AgentDir
	}
	if opts.Auth != nil {
		cfg.Auth = opts.Auth
	}
	mc, err := cfg.DefaultModelConfig()
	if err != nil {
		return nil, err
	}
	if opts.Model != "" {
		found := false
		for _, m := range cfg.Models {
			if m.Name == opts.Model || m.Model == opts.Model {
				mc = m
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("model %q not found", opts.Model)
		}
	}
	reg := opts.Registry
	if reg == nil {
		reg, err = provider.BuildRegistry(cfg.Models, cfg.Auth)
		if err != nil {
			return nil, err
		}
	}
	rt := tools.NewRuntime(tools.Env{Workspace: ws, Trusted: true})
	if opts.NoTools != "all" {
		includeCoding := opts.IncludeCoding
		if opts.NoTools != "builtin" {
			if err := tools.RegisterBuiltinTools(rt, includeCoding); err != nil {
				return nil, err
			}
		}
		for _, t := range opts.CustomTools {
			if err := rt.RegisterOrReplace(t); err != nil {
				return nil, err
			}
		}
		active := opts.Tools
		if len(active) == 0 && opts.NoTools != "builtin" {
			active = append(active, tools.CoreTools...)
			if includeCoding {
				active = append(active, tools.CodingTools...)
			}
			for _, t := range opts.CustomTools {
				active = append(active, t.Def().Name)
			}
		}
		if len(opts.ExcludeTools) > 0 {
			deny := map[string]bool{}
			for _, n := range opts.ExcludeTools {
				deny[n] = true
			}
			filtered := active[:0]
			for _, n := range active {
				if !deny[n] {
					filtered = append(filtered, n)
				}
			}
			active = filtered
		}
		if len(active) > 0 {
			rt.SetActiveTools(active)
		}
	}
	loader := opts.ResourceLoader
	if loader == nil {
		loader = NewDefaultResourceLoader(cfg)
	}
	if err := loader.Reload(); err != nil {
		return nil, err
	}
	cat := loader.Catalog()
	if cat == nil {
		cat, err = discovery.Load(cfg)
		if err != nil {
			return nil, err
		}
	}
	sess := session.NewManager(ws)
	sessPath := opts.SessionPath
	if sessPath == "" {
		sessPath = session.NewSessionPath(cfg.SessionsRoot(), ws)
	}
	svc := agent.NewService(cfg, reg, rt, sess, sessPath, mc, cat, nil)
	if opts.StreamFn != nil {
		svc.StreamFn = opts.StreamFn
	}
	if opts.ThinkingLevel != "" {
		svc.SetThinkingLevel(opts.ThinkingLevel)
	}
	if opts.Resume && opts.SessionPath != "" {
		if err := svc.OpenSession(opts.SessionPath); err != nil {
			return nil, err
		}
		sessPath = opts.SessionPath
	} else if opts.Continue {
		if p, err := svc.ContinueSession(); err == nil && p != "" {
			sessPath = p
		}
	}
	_ = svc.EmitSessionStart()
	diags := loader.Diagnostics()
	return &Session{
		Service: svc, Config: cfg, Registry: reg, SessPath: sessPath,
		ResourceLoader: loader, Diagnostics: diags,
	}, nil
}

// Prompt выполняет один ход пользователя. Caller владеет временем жизни канала events.
func (s *Session) Prompt(ctx context.Context, text string, events chan<- agent.Event) error {
	return s.Service.Prompt(ctx, text, "", events)
}

// Steer ставит steering-сообщение в очередь во время стрима (или начинает ход).
func (s *Session) Steer(ctx context.Context, text string, events chan<- agent.Event) error {
	return s.Service.Prompt(ctx, text, "steer", events)
}

// FollowUp ставит follow-up в очередь во время стрима (или начинает ход).
func (s *Session) FollowUp(ctx context.Context, text string, events chan<- agent.Event) error {
	return s.Service.Prompt(ctx, text, "followUp", events)
}

// Abort отменяет активный run.
func (s *Session) Abort() {
	s.Service.Abort()
}

// Compact запускает компактирование контекста.
func (s *Session) Compact(ctx context.Context) error {
	_, err := s.Service.Compact(ctx)
	return err
}

// SetModel переключает активную модель по имени в каталоге.
func (s *Session) SetModel(name string) error {
	return s.Service.SetModelByName(name)
}

// CycleModel переходит к следующей настроенной модели.
func (s *Session) CycleModel() (string, error) {
	return s.Service.CycleModel()
}

// SetThinkingLevel задаёт уровень reasoning для последующих ходов.
func (s *Session) SetThinkingLevel(level string) {
	s.Service.SetThinkingLevel(level)
}

// SessionTree возвращает in-memory менеджер сессии.
func (s *Session) SessionTree() *session.Manager {
	return s.Service.Sessions
}

// Model возвращает конфиг активной модели.
func (s *Session) Model() ai.ModelConfig {
	return s.Service.ActiveModelConfig()
}

// ReloadResources пересканирует skills/prompts через ResourceLoader.
func (s *Session) ReloadResources() error {
	if s.ResourceLoader == nil {
		return nil
	}
	if err := s.ResourceLoader.Reload(); err != nil {
		return err
	}
	s.Diagnostics = s.ResourceLoader.Diagnostics()
	return nil
}
