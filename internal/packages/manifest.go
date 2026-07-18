package packages

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ManifestName = "package.json"

type Manifest struct {
	Name     string         `json:"name"`
	Version  string         `json:"version"`
	Keywords []string       `json:"keywords,omitempty"`
	Stell    StellResources `json:"stell"`
}

type StellResources struct {
	Extensions []string `json:"extensions,omitempty"`
	Skills     []string `json:"skills,omitempty"`
	Prompts    []string `json:"prompts,omitempty"`
	Themes     []string `json:"themes,omitempty"`
	Include    []string `json:"include,omitempty"`
}

func LoadManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, ManifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ManifestName, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", ManifestName, err)
	}
	if m.Name == "" || m.Version == "" {
		return nil, fmt.Errorf("package.json requires name and version")
	}
	return &m, nil
}

func (m *Manifest) ResourceDirs(root, kind string) []string {
	var rel []string
	switch kind {
	case "skills":
		rel = m.Stell.Skills
	case "prompts":
		rel = m.Stell.Prompts
	case "themes":
		rel = m.Stell.Themes
	case "extensions":
		rel = m.Stell.Extensions
	}
	var out []string
	for _, r := range rel {
		p := filepath.Join(root, filepath.Clean(r))
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			if len(m.Stell.Include) == 0 {
				out = append(out, p)
				continue
			}
			matches, _ := filepath.Glob(filepath.Join(p, "*"))
			for _, match := range matches {
				base := filepath.Base(match)
				if MatchInclude(m.Stell.Include, base) {
					out = append(out, match)
				}
			}
		}
	}
	return out
}

func (m *Manifest) IsStellPackage() bool {
	for _, k := range m.Keywords {
		if k == "stell-package" {
			return true
		}
	}
	return len(m.Stell.Skills) > 0 || len(m.Stell.Prompts) > 0 || len(m.Stell.Extensions) > 0
}

func ParseSource(raw string) (Source, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Source{}, fmt.Errorf("empty package source")
	}
	if strings.HasPrefix(raw, "npm:") {
		return Source{}, fmt.Errorf("npm sources are not supported; use git:URL[@ref]")
	}
	if strings.HasPrefix(raw, "git:") {
		rest := strings.TrimPrefix(raw, "git:")
		ref := ""
		if i := strings.LastIndex(rest, "@"); i >= 0 {
			ref = rest[i+1:]
			rest = rest[:i]
		}
		if rest == "" {
			return Source{}, fmt.Errorf("invalid git source %q", raw)
		}
		if !strings.HasPrefix(rest, "http://") && !strings.HasPrefix(rest, "https://") {
			rest = "https://" + rest
		}
		return Source{Kind: "git", Path: rest, Ref: ref, Raw: raw}, nil
	}
	return Source{Kind: "local", Path: raw, Raw: raw}, nil
}

type Source struct {
	Kind string
	Path string
	Ref  string
	Raw  string
}
