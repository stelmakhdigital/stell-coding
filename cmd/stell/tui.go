package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"stell/coding-agent/internal/agent"
	"stell/coding-agent/internal/extensions"
	"stell/coding-agent/internal/tui"
)

func runTUI(workspace, modelName string, noSession bool, bopts bootstrapOpts) int {
	app, err := bootstrap(workspace, modelName, noSession, bopts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}

	grantCh := make(chan extensions.GrantRequest, 4)
	if app.GrantBroker != nil {
		app.GrantBroker.SetEmitter(func(req extensions.GrantRequest) {
			grantCh <- req
		})
		app.GrantBroker.SetStderrFallback(extensions.StderrGrantPrompt)
	}

	uiCh := make(chan extensions.UIRequest, 4)
	extPromptCh := make(chan agent.ExternalPrompt, 4)
	if app.Extensions != nil {
		app.Extensions.SetHost(app.Service)
		app.Extensions.SetExtensionErrorEmitter(func(extension, message string) {
			fmt.Fprintf(os.Stderr, "extension_error %s: %s\n", extension, message)
		})
		app.Extensions.SetUIProtocol(extensions.NewUIProtocol(func(req extensions.UIRequest) {
			select {
			case uiCh <- req:
			default:
			}
		}))
	}
	app.Service.SetExternalPromptCh(extPromptCh)
	app.Service.WireCompactionNotices()

	workflowCh := make(chan map[string]any, 8)
	if app.Extensions != nil {
		app.Extensions.SetWorkflowNotify(func(method string, params map[string]any) {
			data := map[string]any{}
			for k, v := range params {
				data[k] = v
			}
			data["_method"] = method
			select {
			case workflowCh <- data:
			default:
			}
		})
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = tui.Run(ctx, tui.Options{
		Service:          app.Service,
		Config:           app.Config,
		GrantCh:          grantCh,
		UICh:             uiCh,
		WorkflowCh:       workflowCh,
		ExternalPromptCh: extPromptCh,
	})
	app.Shutdown()
	printResumeHint(os.Args[0], app.Service, noSession, workspace)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 130
		}
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	return 0
}
