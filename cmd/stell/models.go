package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"stell/coding-agent/internal/config"
	"github.com/stelmakhdigital/ai/provider/discover"
)

func runModels(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: stell models list|discover|add|probe")
		return 2
	}
	switch args[0] {
	case "list":
		return modelsList()
	case "discover":
		return modelsDiscover()
	case "add":
		return modelsAdd(args[1:])
	case "probe":
		return modelsProbe(args[1:])
	default:
		fmt.Fprintln(os.Stderr, "unknown models command:", args[0])
		return 2
	}
}

func modelsList() int {
	ws, _ := os.Getwd()
	cfg, err := config.Load(ws)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	ctx := context.Background()
	for _, mc := range cfg.Models {
		status := "ok"
		if mc.Local {
			ep := discover.Endpoint{ProviderID: mc.ProviderID, BaseURL: mc.APIBase, API: mc.APIType}
			if ep.BaseURL == "" {
				ep = discover.DefaultOllama()
			}
			res := discover.Probe(ctx, ep)
			if !res.OK {
				status = "offline"
			} else {
				status = "local"
			}
		} else if !config.HasAuthConfigured(cfg.Auth, mc) {
			status = "needs auth"
		}
		line := fmt.Sprintf("%s  provider=%s model=%s  [%s]", mc.Name, mc.Provider, mc.Model, status)
		if mc.APIBase != "" {
			line += "  base=" + mc.APIBase
		}
		fmt.Println(line)
	}
	return 0
}

func modelsDiscover() int {
	ctx := context.Background()
	found, err := discover.Discover(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	if len(found) == 0 {
		fmt.Println("no local models discovered (is Ollama or LM Studio running?)")
		return 0
	}
	for _, m := range found {
		fmt.Printf("%s  provider=%s  base=%s\n", m.ID, m.ProviderID, m.BaseURL)
	}
	return 0
}

func modelsAdd(args []string) int {
	fs := flag.NewFlagSet("models add", flag.ExitOnError)
	providerID := fs.String("provider", "ollama", "provider id in models.json")
	modelID := fs.String("id", "", "model id (required)")
	baseURL := fs.String("base-url", "", "API base URL")
	displayName := fs.String("name", "", "display name")
	_ = fs.Parse(args)
	if *modelID == "" {
		fmt.Fprintln(os.Stderr, "usage: stell models add --id <model> [--provider ollama] [--base-url URL]")
		return 2
	}
	path, err := config.GlobalModelsPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	mf := config.ModelsFile{Format: "providers", Providers: map[string]config.ProviderConfig{}}
	if existing, err := config.LoadModelsFile(path); err == nil {
		mf = existing
		if mf.Providers == nil {
			mf.Providers = map[string]config.ProviderConfig{}
		}
	}
	defaults := discover.DefaultOllama()
	if *providerID != "ollama" {
		defaults = discover.Endpoint{ProviderID: *providerID, BaseURL: "http://localhost:1234/v1", API: "openai-completions"}
	}
	if *baseURL != "" {
		defaults.BaseURL = *baseURL
	}
	f := false
	pcDefaults := config.ProviderConfig{
		BaseURL: defaults.BaseURL,
		API:     defaults.API,
		APIKey:  "ollama",
		Compat:  config.CompatSettings{SupportsDeveloperRole: &f, SupportsReasoningEffort: &f},
	}
	entry := config.ProviderModelEntry{ID: *modelID}
	if *displayName != "" {
		entry.Name = *displayName
	}
	if err := mf.AddProviderModel(*providerID, entry, pcDefaults); err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	if err := config.SaveModelsFile(path, mf); err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	fmt.Printf("added %q to %s\n", *modelID, path)
	return 0
}

func modelsProbe(args []string) int {
	fs := flag.NewFlagSet("models probe", flag.ExitOnError)
	baseURL := fs.String("base-url", "", "endpoint base URL")
	providerID := fs.String("provider", "ollama", "provider label")
	_ = fs.Parse(args)
	ep := discover.DefaultOllama()
	if *baseURL != "" {
		ep.BaseURL = *baseURL
	}
	ep.ProviderID = *providerID
	res := discover.Probe(context.Background(), ep)
	if !res.OK {
		fmt.Printf("%s  offline: %s\n", ep.BaseURL, res.Error)
		return 1
	}
	fmt.Printf("%s  ok  models=%s\n", ep.BaseURL, strings.Join(res.Models, ", "))
	return 0
}
