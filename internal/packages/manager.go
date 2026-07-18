package packages

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type State string

const (
	StateEnabled State = "enabled"
)

type Record struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Source      string    `json:"source"`
	InstallPath string    `json:"installPath"`
	State       State     `json:"state"`
	InstalledAt time.Time `json:"installedAt"`
}

type stateFile struct {
	Packages map[string]Record `json:"packages"`
}

type Manager struct {
	GlobalDir  string
	ProjectDir string
	Scope      string // global | project
}

func NewManager(globalDir, projectDir, scope string) *Manager {
	if scope == "" {
		scope = "project"
	}
	return &Manager{GlobalDir: globalDir, ProjectDir: projectDir, Scope: scope}
}

func (m *Manager) installRoot() string {
	if m.Scope == "global" {
		return filepath.Join(m.GlobalDir, "packages")
	}
	return filepath.Join(m.ProjectDir, ".stell", "packages")
}

func (m *Manager) statePath() string {
	return filepath.Join(m.installRoot(), "state.json")
}

func (m *Manager) Install(ctx context.Context, rawSource string) (*Record, error) {
	src, err := ParseSource(rawSource)
	if err != nil {
		return nil, err
	}
	if src.Kind == "local" {
		abs, err := filepath.Abs(src.Path)
		if err != nil {
			return nil, err
		}
		src.Path = abs
	}
	manifest, peekDir, err := m.peekManifest(ctx, src)
	if err != nil {
		return nil, err
	}
	if !manifest.IsStellPackage() {
		return nil, fmt.Errorf("not a stell package (missing stell key or stell-package keyword)")
	}
	dest := filepath.Join(m.installRoot(), manifest.Name)
	if err := os.MkdirAll(m.installRoot(), 0o755); err != nil {
		return nil, err
	}
	_ = os.RemoveAll(dest)
	if err := m.installSource(ctx, src, dest); err != nil {
		return nil, err
	}
	if _, err := LoadManifest(dest); err != nil {
		_ = os.RemoveAll(dest)
		return nil, err
	}
	_ = peekDir
	rec := Record{
		Name: manifest.Name, Version: manifest.Version, Source: rawSource,
		InstallPath: dest, State: StateEnabled, InstalledAt: time.Now().UTC(),
	}
	st, err := loadState(m.statePath())
	if err != nil {
		return nil, err
	}
	st.Packages[rec.Name] = rec
	if err := saveState(m.statePath(), st); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (m *Manager) List() ([]Record, error) {
	st, err := loadState(m.statePath())
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(st.Packages))
	for _, r := range st.Packages {
		if r.State == StateEnabled {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *Manager) Remove(name string) error {
	st, err := loadState(m.statePath())
	if err != nil {
		return err
	}
	rec, ok := st.Packages[name]
	if !ok {
		return fmt.Errorf("package %q not installed", name)
	}
	delete(st.Packages, name)
	if err := saveState(m.statePath(), st); err != nil {
		return err
	}
	return os.RemoveAll(rec.InstallPath)
}

func (m *Manager) Update(ctx context.Context, name string) error {
	st, err := loadState(m.statePath())
	if err != nil {
		return err
	}
	var names []string
	if name != "" {
		if _, ok := st.Packages[name]; !ok {
			return fmt.Errorf("package %q not installed", name)
		}
		names = []string{name}
	} else {
		for n := range st.Packages {
			names = append(names, n)
		}
	}
	for _, n := range names {
		rec := st.Packages[n]
		if _, err := m.Install(ctx, rec.Source); err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
	}
	return nil
}

func (m *Manager) EnabledResourceDirs(kind string) ([]string, error) {
	recs, err := m.List()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, r := range recs {
		manifest, err := LoadManifest(r.InstallPath)
		if err != nil {
			continue
		}
		out = append(out, manifest.ResourceDirs(r.InstallPath, kind)...)
	}
	return out, nil
}

func (m *Manager) installGit(ctx context.Context, src Source, dest string) error {
	return gitClone(ctx, src.Path, src.Ref, dest)
}

func (m *Manager) peekManifest(ctx context.Context, src Source) (*Manifest, string, error) {
	switch src.Kind {
	case "local":
		manifest, err := LoadManifest(src.Path)
		return manifest, src.Path, err
	case "git":
		tmp, err := os.MkdirTemp("", "stell-pkg-peek-")
		if err != nil {
			return nil, "", err
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		if err := m.installGit(ctx, src, tmp); err != nil {
			return nil, "", err
		}
		manifest, err := LoadManifest(tmp)
		return manifest, tmp, err
	default:
		return nil, "", fmt.Errorf("unknown source kind %q", src.Kind)
	}
}

func (m *Manager) installSource(ctx context.Context, src Source, dest string) error {
	switch src.Kind {
	case "local":
		return copyDir(src.Path, dest)
	case "git":
		return m.installGit(ctx, src, dest)
	default:
		return fmt.Errorf("unknown source kind %q", src.Kind)
	}
}

func loadState(path string) (stateFile, error) {
	var s stateFile
	s.Packages = map[string]Record{}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	if s.Packages == nil {
		s.Packages = map[string]Record{}
	}
	return s, nil
}

func saveState(path string, s stateFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, fi.Mode())
		}
		return copyFile(path, target, fi.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
