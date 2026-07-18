package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"stell/coding-agent/internal/agent"
	"stell/coding-agent/internal/rpc"
	"stell/coding-agent/internal/update"

	_ "github.com/stelmakhdigital/ai/provider/all"
)

func main() {
	initOfflineFromArgs(os.Args[1:])
	if hasVersionFlag(os.Args[1:]) {
		printVersion()
		os.Exit(0)
	}
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "install":
			os.Exit(runInstall(os.Args[2:]))
		case "config":
			os.Exit(runConfig(os.Args[2:]))
		case "pkg":
			os.Exit(runPkg(os.Args[2:]))
		case "update":
			os.Exit(runUpdate(os.Args[2:]))
		case "login":
			os.Exit(runLogin(os.Args[2:]))
		case "logout":
			os.Exit(runLogout(os.Args[2:]))
		case "models":
			os.Exit(runModels(os.Args[2:]))
		case "share-hf":
			os.Exit(runShareHF(os.Args[2:]))
		}
	}
	os.Exit(run())
}

func run() int {
	printFlag := flag.Bool("p", false, "print mode: run one prompt and exit")
	once := flag.String("once", "", "alias for -p")
	mode := flag.String("mode", "text", "output mode: text, json, or rpc")
	noSession := flag.Bool("no-session", false, "do not persist session")
	workspace := flag.String("workspace", "", "workspace root (default cwd)")
	modelName := flag.String("model", "", "model name from models.json")
	continueFlag := flag.Bool("c", false, "continue latest session or create new")
	continueLong := flag.Bool("continue", false, "alias for -c")
	resumeFlag := flag.Bool("r", false, "resume latest session")
	resumeLong := flag.Bool("resume", false, "alias for -r")
	sessionFlag := flag.String("session", "", "open specific session file (.jsonl)")
	extDir := flag.String("e", "", "extra extension directory")
	apiKey := flag.String("api-key", "", "override API key for provider")
	autoApprove := flag.Bool("approve", false, "auto-approve bash (non-interactive)")
	noApprove := flag.Bool("no-approve", false, "deny bash without prompt")
	toolsFlag := flag.String("tools", "", "comma-separated tool allowlist (default: read,write,edit,bash)")
	excludeTools := flag.String("exclude-tools", "", "comma-separated tools to exclude")
	noTools := flag.Bool("no-tools", false, "expose no tools to the LLM")
	noBuiltinTools := flag.Bool("no-builtin-tools", false, "do not register built-in tools")
	codingTools := flag.Bool("coding-tools", false, "include grep,find,ls in the default tool set")
	offline := flag.Bool("offline", false, "disable startup network operations")
	versionFlag := flag.Bool("version", false, "print version")
	shortVersion := flag.Bool("v", false, "print version")
	flag.Parse()

	if *offline {
		update.EnableOffline()
	}
	if *versionFlag || *shortVersion {
		printVersion()
		return 0
	}

	bopts := bootstrapOpts{
		continueSession: *continueFlag || *continueLong,
		resumeSession:   *resumeFlag || *resumeLong,
		sessionPath:     *sessionFlag,
		extensionDir:    *extDir,
		apiKey:          *apiKey,
		autoApprove:     *autoApprove,
		noApprove:       *noApprove,
		interactive:     isInteractive() && *mode != "rpc" && *once == "" && !*printFlag,
		tools:           *toolsFlag,
		excludeTools:    *excludeTools,
		noTools:         *noTools,
		noBuiltinTools:  *noBuiltinTools,
		includeCoding:   *codingTools,
	}

	if *mode == "rpc" {
		return runRPC(*workspace, *modelName, *noSession, bopts)
	}

	prompt := *once
	args := flag.Args()
	if prompt == "" && *printFlag && len(args) > 0 {
		prompt = args[0]
	}
	if prompt != "" {
		return runPrint(*workspace, *modelName, *noSession, prompt, *mode, bopts)
	}

	return runTUI(*workspace, *modelName, *noSession, bopts)
}

func runPrint(workspace, modelName string, noSession bool, prompt, mode string, bopts bootstrapOpts) int {
	app, err := bootstrap(workspace, modelName, noSession, bopts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}

	ag := &agent.Agent{
		Config:   app.Config,
		Registry: app.Registry,
		Tools:    app.Tools,
		Sessions: app.Sessions,
		SessPath: app.SessPath,
		Model:    app.Model,
		Catalog:  app.Catalog,
		Hooks:    app.Service.Hooks,
		StreamFn: app.Service.StreamFn,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer app.Shutdown()

	events := make(chan agent.Event, 64)
	if mode == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		emit := func(obj map[string]any) {
			obj["sessionId"] = app.Sessions.Header.ID
			_ = enc.Encode(obj)
		}
		ag.CompactionEmitter = func(start bool, reason string, info any) {
			if start {
				rpc.EmitCompactionEvent(emit, true, reason)
			} else {
				rpc.EmitCompactionEvent(emit, false, info)
			}
		}
		go ag.Run(ctx, prompt, events)
		return rpc.StreamAgentEvents(emit, events)
	}
	go ag.Run(ctx, prompt, events)
	return runText(events)
}

func runRPC(workspace, modelName string, noSession bool, bopts bootstrapOpts) int {
	app, err := bootstrap(workspace, modelName, noSession, bopts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}

	srv := rpc.NewServer(app.Service, app.Config)
	srv.WireGrantBroker(app.GrantBroker)
	srv.WireUIProtocol(app.Extensions)
	srv.WireCompactionEmitter()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer app.Shutdown()

	if err := srv.Serve(ctx); err != nil && err != context.Canceled {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	return 0
}

func runText(events <-chan agent.Event) int {
	exitCode := 0
	for ev := range events {
		switch ev.Type {
		case agent.EventToken:
			fmt.Print(ev.Token)
		case agent.EventToolCall:
			if ev.ToolCall != nil {
				fmt.Fprintf(os.Stderr, "\n[tool] %s\n", ev.ToolCall.Name)
			}
		case agent.EventToolResult:
			if ev.ToolResult != nil && ev.ToolResult.Error != "" {
				fmt.Fprintf(os.Stderr, "[tool error] %s\n", ev.ToolResult.Error)
			}
		case agent.EventError:
			fmt.Fprintln(os.Stderr, "stell:", ev.Err)
			exitCode = 1
		case agent.EventDone:
			fmt.Println()
		}
	}
	return exitCode
}
