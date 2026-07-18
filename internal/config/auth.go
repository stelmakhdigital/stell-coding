package config

import (
	"github.com/stelmakhdigital/stell-ai"
	"github.com/stelmakhdigital/stell-ai/auth/oauth"
)

// Типы Auth живут в слое ai; алиасы сохраняют call sites coding-agent.
type Auth = ai.Auth
type AuthEntry = ai.AuthEntry

func LoadAuth(globalDir string) (*Auth, error) { return ai.LoadAuth(globalDir) }

func ResolveAPIKey(auth *Auth, mc ModelConfig) (string, error) {
	return ai.ResolveAPIKey(auth, mc)
}

func OAuthBetaHeader() string { return ai.OAuthBetaHeader() }

func SavePendingOAuthState(globalDir string, state oauth.PendingState) error {
	return ai.SavePendingOAuthState(globalDir, state)
}

func LoadPendingOAuthState(globalDir string) (oauth.PendingState, bool, error) {
	return ai.LoadPendingOAuthState(globalDir)
}

func ClearPendingOAuthState(globalDir string) {
	ai.ClearPendingOAuthState(globalDir)
}
