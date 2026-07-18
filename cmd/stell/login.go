package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/stelmakhdigital/stell-ai/auth/oauth"
	"github.com/stelmakhdigital/stell-coding/internal/config"
)

func runLogin(args []string) int {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	device := fs.Bool("device", false, "OAuth device / browser login")
	legacy := fs.Bool("device-legacy", false, "legacy OAuth callback MVP (localhost :8765)")
	_ = fs.Parse(args)
	rest := fs.Args()

	provider := ""
	if len(rest) > 0 {
		provider = strings.TrimSpace(rest[0])
	}
	globalDir, err := config.GlobalDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	useLegacy := *legacy || os.Getenv("STELL_OAUTH_LEGACY") == "1"
	if *device {
		if useLegacy {
			if provider == "" {
				provider = "openai"
			}
			return runOAuthLoginLegacy(provider, globalDir)
		}
		if provider == "" {
			provider = promptOAuthProvider()
			if provider == "" {
				return 2
			}
		}
		switch provider {
		case "anthropic":
			return runAnthropicOAuth(globalDir)
		case "openai", "openai-codex":
			return runOpenAIOAuth(globalDir, provider)
		case "github-copilot":
			return runCopilotOAuth(globalDir)
		case "radius":
			return runRadiusOAuth(globalDir)
		default:
			fmt.Fprintf(os.Stderr, "stell: native OAuth not supported for %q; use --device-legacy or API key login\n", provider)
			fmt.Fprintln(os.Stderr, "supported: anthropic | openai | openai-codex | github-copilot | radius")
			return 2
		}
	}

	if provider == "" {
		provider = "openai"
	}
	return runAPIKeyLogin(provider, globalDir)
}

func promptOAuthProvider() string {
	fmt.Fprintln(os.Stderr, "Select provider for OAuth login:")
	fmt.Fprintln(os.Stderr, "  1) anthropic (Claude Pro/Max PKCE)")
	fmt.Fprintln(os.Stderr, "  2) openai (ChatGPT device flow)")
	fmt.Fprintln(os.Stderr, "  3) openai-codex (Codex / ChatGPT device flow)")
	fmt.Fprintln(os.Stderr, "  4) github-copilot (GitHub device flow)")
	fmt.Fprintln(os.Stderr, "  5) radius (Radius gateway device flow)")
	fmt.Fprint(os.Stderr, "Choice [1-5]: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	switch strings.TrimSpace(line) {
	case "1", "anthropic":
		return "anthropic"
	case "2", "openai":
		return "openai"
	case "3", "openai-codex", "codex":
		return "openai-codex"
	case "4", "github-copilot", "copilot":
		return "github-copilot"
	case "5", "radius":
		return "radius"
	default:
		return "anthropic"
	}
}

func runAPIKeyLogin(provider, globalDir string) int {
	fmt.Fprintf(os.Stderr, "stell login: store API key for provider %q\n", provider)
	fmt.Fprint(os.Stderr, "API key: ")
	reader := bufio.NewReader(os.Stdin)
	key, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	key = strings.TrimSpace(key)
	if key == "" {
		fmt.Fprintln(os.Stderr, "empty key")
		return 1
	}
	auth, err := config.LoadAuth(globalDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	auth.SetProviderKey(provider, key)
	if err := auth.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "saved credentials for %q in %s\n", provider, authPath(globalDir))
	return 0
}

func runAnthropicOAuth(globalDir string) int {
	ctx := context.Background()
	authURL, verifier, state, err := oauth.AnthropicLogin()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := config.SavePendingOAuthState(globalDir, oauth.NewPendingState("anthropic", state, verifier)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer config.ClearPendingOAuthState(globalDir)

	fmt.Fprintf(os.Stderr, "Open this URL in your browser:\n%s\n", authURL)
	_ = oauth.OpenBrowser(authURL)
	fmt.Fprint(os.Stderr, "Paste authorization code from callback page: ")
	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	code = strings.TrimSpace(code)
	if code == "" {
		fmt.Fprintln(os.Stderr, "empty code")
		return 1
	}

	pending, ok, err := config.LoadPendingOAuthState(globalDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !ok || pending.Provider != "anthropic" || !pending.Valid() {
		fmt.Fprintln(os.Stderr, "no valid pending OAuth state; restart login")
		return 1
	}

	tokens, err := oauth.AnthropicExchangeCode(ctx, code, pending.CodeVerifier)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	auth, err := config.LoadAuth(globalDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	auth.SetOAuthFromTokenSet("anthropic", tokens)
	if err := auth.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "saved Anthropic OAuth credentials in %s\n", authPath(globalDir))
	return 0
}

func runOpenAIOAuth(globalDir, provider string) int {
	if provider == "" {
		provider = "openai"
	}
	ctx := context.Background()
	deviceID, userCode, verifyURL, interval, err := oauth.OpenAIDeviceStart(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "Open %s and enter code: %s\n", verifyURL, userCode)
	_ = oauth.OpenBrowser(verifyURL)
	fmt.Fprintln(os.Stderr, "Waiting for authorization…")

	tokens, err := oauth.OpenAIDevicePoll(ctx, deviceID, userCode, interval)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	auth, err := config.LoadAuth(globalDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	auth.SetOAuthFromTokenSet(provider, tokens)
	if provider == "openai-codex" {
		// Также сохраняем под openai для общих учётных данных ChatGPT.
		auth.SetOAuthFromTokenSet("openai", tokens)
	}
	if err := auth.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "saved %s OAuth credentials in %s\n", provider, authPath(globalDir))
	return 0
}

func runCopilotOAuth(globalDir string) int {
	ctx := context.Background()
	deviceCode, userCode, verifyURL, interval, err := oauth.CopilotDeviceStart(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "Open %s and enter code: %s\n", verifyURL, userCode)
	_ = oauth.OpenBrowser(verifyURL)
	fmt.Fprintln(os.Stderr, "Waiting for GitHub authorization…")

	tokens, err := oauth.CopilotDevicePoll(ctx, deviceCode, interval)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	auth, err := config.LoadAuth(globalDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	auth.SetOAuthFromTokenSet("github-copilot", tokens)
	if err := auth.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "saved GitHub Copilot OAuth credentials in %s\n", authPath(globalDir))
	return 0
}

func runRadiusOAuth(globalDir string) int {
	ctx := context.Background()
	deviceCode, userCode, verifyURL, interval, _, oauthCfg, err := oauth.RadiusDeviceStart(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "Open %s and enter code: %s\n", verifyURL, userCode)
	_ = oauth.OpenBrowser(verifyURL)
	fmt.Fprintln(os.Stderr, "Waiting for Radius authorization…")

	tokens, err := oauth.RadiusDevicePoll(ctx, oauthCfg, deviceCode, interval)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	auth, err := config.LoadAuth(globalDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	auth.SetOAuthFromTokenSet("radius", tokens)
	if err := auth.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := oauth.RadiusLoadGatewayConfig(ctx, oauth.RadiusGateway(), tokens.AccessToken); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load Radius model catalog: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "saved Radius OAuth credentials in %s\n", authPath(globalDir))
	return 0
}

func runOAuthLoginLegacy(provider, globalDir string) int {
	fmt.Fprintf(os.Stderr, "stell login --device-legacy: OAuth for provider %q\n", provider)
	fmt.Fprintf(os.Stderr, "Callback: http://127.0.0.1:8765/callback?token=... (10s timeout)\n")

	tokenCh := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if t := r.URL.Query().Get("token"); t != "" {
			tokenCh <- t
		} else if c := r.URL.Query().Get("code"); c != "" {
			tokenCh <- c
		}
		_, _ = fmt.Fprint(w, "OK — return to terminal")
	})
	srv := &http.Server{Addr: "127.0.0.1:8765", Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	defer func() { _ = srv.Shutdown(shutdownCtx) }()

	fmt.Fprint(os.Stderr, "Access token (or wait for callback): ")
	go func() {
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" {
			tokenCh <- line
		}
	}()

	var token string
	select {
	case token = <-tokenCh:
	case <-time.After(10 * time.Second):
	}

	if token == "" {
		fmt.Fprintln(os.Stderr, "no token received")
		return 1
	}
	auth, err := config.LoadAuth(globalDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	auth.SetOAuthToken(provider, token)
	if err := auth.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "saved OAuth token for %q in %s\n", provider, authPath(globalDir))
	return 0
}

func authPath(globalDir string) string {
	return globalDir + "/auth.json"
}
