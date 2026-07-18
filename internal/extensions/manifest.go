package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ManifestJSON = "manifest.json"
	ManifestYAML = "manifest.yaml"
	TypeProcess  = "process-jsonrpc"
)

type Manifest struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Command     []string          `json:"command"`
	Hooks       []string          `json:"hooks,omitempty"`
	Commands    []ManifestCommand `json:"commands,omitempty"`
	Shortcuts   []ManifestShortcut  `json:"shortcuts,omitempty"`
	Flags       []ManifestFlag      `json:"flags,omitempty"`
	Permissions ExtPermissions    `json:"permissions,omitempty"`
}

type ManifestCommand struct {
	Slash       string `json:"slash"`
	Description string `json:"description"`
}

type ManifestShortcut struct {
	Key    string `json:"key"`
	Action string `json:"action"`
}

type ManifestFlag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type,omitempty"`
	Default     any    `json:"default,omitempty"`
}

func (m *Manifest) Subscribes(hook string) bool {
	for _, h := range m.Hooks {
		if h == "*" || h == hook {
			return true
		}
	}
	return false
}

func LoadManifest(dir string) (*Manifest, error) {
	for _, name := range []string{ManifestJSON, ManifestYAML} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		var m Manifest
		if name == ManifestJSON {
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, fmt.Errorf("parse %s: %w", name, err)
			}
		} else {
			m, err = parseSimpleYAML(data)
			if err != nil {
				return nil, err
			}
		}
		if m.Type == "" {
			m.Type = TypeProcess
		}
		if m.Name == "" {
			return nil, fmt.Errorf("extension manifest requires name")
		}
		return &m, nil
	}
	return nil, fmt.Errorf("no manifest in %s", dir)
}

func parseSimpleYAML(data []byte) (Manifest, error) {
	var m Manifest
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "name":
			m.Name = unquote(val)
		case "type":
			m.Type = unquote(val)
		case "command":
			m.Command = parseYAMLList(val)
		case "hooks":
			m.Hooks = parseYAMLList(val)
		}
	}
	return m, nil
}

func unquote(s string) string {
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

func parseYAMLList(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") {
		if s != "" {
			return []string{unquote(s)}
		}
		return nil
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = unquote(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
