package trust

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type File struct {
	TrustedWorkspaces []string `json:"trustedWorkspaces,omitempty"`
	BashAutoApprove   bool     `json:"bashAutoApprove,omitempty"`
}

func globalPath(globalDir string) string {
	return filepath.Join(globalDir, "trust.json")
}

func workspacePath(workspace string) string {
	return filepath.Join(workspace, ".stell", "trust.json")
}

func LoadGlobal(globalDir string) (*File, error) {
	return loadFile(globalPath(globalDir))
}

func LoadWorkspace(workspace string) (*File, error) {
	return loadFile(workspacePath(workspace))
}

func LoadMerged(globalDir, workspace string) (*File, error) {
	g, err := LoadGlobal(globalDir)
	if err != nil {
		return nil, err
	}
	w, err := LoadWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	return merge(g, w), nil
}

func merge(a, b *File) *File {
	if a == nil {
		a = &File{}
	}
	if b == nil {
		return a
	}
	out := *a
	if b.BashAutoApprove {
		out.BashAutoApprove = true
	}
	seen := map[string]bool{}
	for _, w := range out.TrustedWorkspaces {
		seen[w] = true
	}
	for _, w := range b.TrustedWorkspaces {
		if !seen[w] {
			out.TrustedWorkspaces = append(out.TrustedWorkspaces, w)
		}
	}
	return &out
}

func loadFile(path string) (*File, error) {
	f := &File{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return f, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, f); err != nil {
		return nil, err
	}
	return f, nil
}

func SaveGlobal(globalDir string, f *File) error {
	return saveFile(globalPath(globalDir), f)
}

func SaveWorkspace(workspace string, f *File) error {
	return saveFile(workspacePath(workspace), f)
}

func saveFile(path string, f *File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func (f *File) IsTrusted(workspace string) bool {
	if f == nil {
		return false
	}
	if f.BashAutoApprove {
		return true
	}
	for _, w := range f.TrustedWorkspaces {
		if w == workspace {
			return true
		}
	}
	return false
}

func TrustWorkspace(globalDir, workspace string) error {
	return TrustWorkspaceOpts(globalDir, workspace, false)
}

// TrustWorkspaceOpts сохраняет решение о доверии; parent=true также доверяет родительский каталог.
func TrustWorkspaceOpts(globalDir, workspace string, parent bool) error {
	f, err := LoadGlobal(globalDir)
	if err != nil {
		return err
	}
	add := func(path string) {
		for _, w := range f.TrustedWorkspaces {
			if w == path {
				return
			}
		}
		f.TrustedWorkspaces = append(f.TrustedWorkspaces, path)
	}
	add(workspace)
	if parent {
		if p := filepath.Dir(workspace); p != "" && p != workspace && p != "." && p != "/" {
			add(p)
		}
	}
	return SaveGlobal(globalDir, f)
}

// EnsureTrust спрашивает на stderr, если есть project settings и workspace не доверен.
func EnsureTrust(globalDir, workspace string, defaultTrust string, autoApprove, noApprove, interactive bool) (bool, error) {
	if noApprove {
		return false, nil
	}
	if autoApprove {
		return true, nil
	}
	tf, err := LoadMerged(globalDir, workspace)
	if err != nil {
		return false, err
	}
	if tf.IsTrusted(workspace) {
		return true, nil
	}
	switch defaultTrust {
	case "always":
		return true, TrustWorkspace(globalDir, workspace)
	case "never":
		return false, nil
	}
	if !interactive {
		return false, nil
	}
	stellDir := filepath.Join(workspace, ".stell")
	if _, err := os.Stat(stellDir); os.IsNotExist(err) {
		return false, nil
	}
	fmt.Fprintf(os.Stderr, "Trust workspace %q for bash? [y/N]: ", workspace)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.TrimSpace(strings.ToLower(line)) == "y" {
		return true, TrustWorkspace(globalDir, workspace)
	}
	return false, nil
}

// Deprecated: используйте LoadMerged.
func Load(workspace string) (*File, error) {
	return LoadWorkspace(workspace)
}

// Deprecated: используйте SaveWorkspace.
func Save(workspace string, f *File) error {
	return SaveWorkspace(workspace, f)
}
