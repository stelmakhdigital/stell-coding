package config

import (
	"testing"
	"time"

	"github.com/stelmakhdigital/stell-ai"
	"github.com/stelmakhdigital/stell-ai/auth/oauth"
)

func TestOAuthCredentialsRoundTrip(t *testing.T) {
	a := ai.NewAuthForTest()
	a.SetOAuthFromTokenSet("anthropic", oauth.TokenSet{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresIn:    3600,
	})
	e, ok := a.Entry("anthropic")
	if !ok || e.Type != "oauth" || e.Access != "access" || e.Refresh != "refresh" || e.Expires == 0 {
		t.Fatalf("entry: ok=%v %+v", ok, e)
	}
}

func TestResolveOAuthUsesAccessBeforeExpiry(t *testing.T) {
	a := ai.NewAuthForTest()
	a.SetOAuthCredentials("anthropic", "tok", "ref", time.Now().Add(time.Hour).Unix())
	got, err := a.Resolve("anthropic")
	if err != nil || got != "tok" {
		t.Fatalf("resolve=%q err=%v", got, err)
	}
}

func TestResolveAPIKeyPrefersCLIOverride(t *testing.T) {
	a := ai.NewAuthForTest()
	a.SetCLIOverride("from-cli")
	got, err := ResolveAPIKey(a, ModelConfig{Provider: "openai"})
	if err != nil || got != "from-cli" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
