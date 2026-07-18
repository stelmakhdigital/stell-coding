package extensions

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"stell/coding-agent/internal/config"
)

type GrantChecker interface {
	HasGrant(key string) bool
	RequestGrant(ctx context.Context, key string, perms ExtPermissions) (bool, error)
}

type SettingsGrantChecker struct {
	Settings     *config.Settings
	SettingsPath string
	Interactive  bool
}

func NewSettingsGrantChecker(settings *config.Settings, path string, interactive bool) *SettingsGrantChecker {
	return &SettingsGrantChecker{Settings: settings, SettingsPath: path, Interactive: interactive}
}

func (g *SettingsGrantChecker) HasGrant(key string) bool {
	if g.Settings != nil && g.Settings.Security.ExtensionGrants != nil {
		return g.Settings.Security.ExtensionGrants[key]
	}
	return false
}

func (g *SettingsGrantChecker) RequestGrant(ctx context.Context, key string, perms ExtPermissions) (bool, error) {
	_ = ctx
	if g.HasGrant(key) {
		return true, nil
	}
	if !g.Interactive {
		return false, fmt.Errorf("extension %q requires permissions", key)
	}
	ok, err := StderrGrantPrompt(GrantRequest{Key: key, Perms: perms})
	if err != nil || !ok {
		return ok, err
	}
	if g.Settings != nil {
		if g.Settings.Security.ExtensionGrants == nil {
			g.Settings.Security.ExtensionGrants = map[string]bool{}
		}
		g.Settings.Security.ExtensionGrants[key] = true
		if g.SettingsPath != "" {
			_ = config.PersistExtensionGrant(g.SettingsPath, key)
		}
	}
	return true, nil
}

type BrokerGrantChecker struct {
	Broker       *GrantBroker
	Settings     *config.Settings
	SettingsPath string
}

func NewBrokerGrantChecker(broker *GrantBroker, settings *config.Settings, path string) *BrokerGrantChecker {
	return &BrokerGrantChecker{Broker: broker, Settings: settings, SettingsPath: path}
}

func (g *BrokerGrantChecker) HasGrant(key string) bool {
	if g.Settings != nil && g.Settings.Security.ExtensionGrants != nil {
		return g.Settings.Security.ExtensionGrants[key]
	}
	return false
}

func (g *BrokerGrantChecker) RequestGrant(ctx context.Context, key string, perms ExtPermissions) (bool, error) {
	if g.HasGrant(key) {
		return true, nil
	}
	if g.Broker == nil {
		return NewSettingsGrantChecker(g.Settings, g.SettingsPath, false).RequestGrant(ctx, key, perms)
	}
	ok, err := g.Broker.Request(ctx, key, perms)
	if err != nil || !ok {
		return ok, err
	}
	if g.Settings != nil {
		if g.Settings.Security.ExtensionGrants == nil {
			g.Settings.Security.ExtensionGrants = map[string]bool{}
		}
		g.Settings.Security.ExtensionGrants[key] = true
		if g.SettingsPath != "" {
			_ = config.PersistExtensionGrant(g.SettingsPath, key)
		}
	}
	return true, nil
}

func StderrGrantPrompt(req GrantRequest) (bool, error) {
	fmt.Fprintf(os.Stderr, "Extension %q requests permissions:\n", req.Key)
	if req.Perms.Shell {
		fmt.Fprintln(os.Stderr, "  - shell access")
	}
	if req.Perms.Network {
		fmt.Fprintln(os.Stderr, "  - network access")
	}
	for _, p := range req.Perms.Paths {
		fmt.Fprintf(os.Stderr, "  - path: %s\n", p)
	}
	fmt.Fprint(os.Stderr, "Grant? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(line)) != "y" {
		return false, nil
	}
	return true, nil
}
