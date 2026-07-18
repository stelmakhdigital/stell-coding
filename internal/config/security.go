package config

import (
	"encoding/json"
	"os"
)

type SecuritySettings struct {
	ExtensionGrants map[string]bool `json:"extensionGrants,omitempty"`
}

func PersistExtensionGrant(settingsPath, grantKey string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	sec, _ := doc["security"].(map[string]any)
	if sec == nil {
		sec = map[string]any{}
		doc["security"] = sec
	}
	grants, _ := sec["extensionGrants"].(map[string]any)
	if grants == nil {
		grants = map[string]any{}
	}
	grants[grantKey] = true
	sec["extensionGrants"] = grants
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(settingsPath, out, 0o644)
}
