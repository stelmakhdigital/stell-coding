package agent

import (
	"os"
	"strings"

	coreagent "stell/agent"
	"stell/agent/proxy"
)

// StreamFnFromEnv собирает proxy StreamFn из STELL_PROXY_URL / STELL_PROXY_TOKEN.
// Возвращает nil, если STELL_PROXY_URL не задан.
func StreamFnFromEnv() coreagent.StreamFn {
	url := strings.TrimSpace(os.Getenv("STELL_PROXY_URL"))
	if url == "" {
		return nil
	}
	return proxy.StreamProxy(proxy.Options{
		ProxyURL:  url,
		AuthToken: strings.TrimSpace(os.Getenv("STELL_PROXY_TOKEN")),
	})
}
