package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"stell/coding-agent/internal/version"
)

type InstallMethod string

const (
	InstallGoInstall  InstallMethod = "go-install"
	InstallBrew       InstallMethod = "brew"
	InstallLocalBuild InstallMethod = "local-build"
	InstallUnknown    InstallMethod = "unknown"
)

type SelfUpdateCommand struct {
	Command string
	Args    []string
	Display string
}

// DetectInstallMethod определяет способ установки stell по пути executable.
func DetectInstallMethod(execPath string) InstallMethod {
	p := strings.ToLower(filepath.ToSlash(execPath))
	switch {
	case strings.Contains(p, "/cellar/") || strings.Contains(p, "/opt/homebrew/"):
		return InstallBrew
	case isGoInstallPath(p):
		return InstallGoInstall
	case isLocalBuildPath(p):
		return InstallLocalBuild
	default:
		return InstallUnknown
	}
}

func isGoInstallPath(p string) bool {
	if bin := goBinDir(); bin != "" {
		bin = strings.ToLower(filepath.ToSlash(bin))
		if strings.HasPrefix(p, bin+"/") || p == bin {
			return true
		}
	}
	return strings.Contains(p, "/go/bin/") || strings.Contains(p, "/gopath/bin/")
}

func isLocalBuildPath(p string) bool {
	if strings.Contains(p, "/stell/") || strings.Contains(p, "/packages/coding-agent/") {
		return true
	}
	dir := filepath.Dir(p)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}

func goBinDir() string {
	if v := strings.TrimSpace(os.Getenv("GOBIN")); v != "" {
		return v
	}
	out, err := exec.Command("go", "env", "GOBIN").Output()
	if err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return v
		}
	}
	out, err = exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		return ""
	}
	gopath := strings.TrimSpace(string(out))
	if gopath == "" {
		return ""
	}
	first := strings.Split(gopath, string(os.PathListSeparator))[0]
	return filepath.Join(first, "bin")
}

// GetSelfUpdateCommand собирает команду обновления stell или nil, если недоступна.
func GetSelfUpdateCommand(method InstallMethod, modulePath, ver string) (*SelfUpdateCommand, error) {
	if modulePath == "" {
		modulePath = version.DefaultModulePath
	}
	spec := modulePath + "@v" + strings.TrimPrefix(strings.TrimSpace(ver), "v")
	switch method {
	case InstallGoInstall:
		return &SelfUpdateCommand{
			Command: "go",
			Args:    []string{"install", spec},
			Display: "go install " + spec,
		}, nil
	case InstallBrew:
		return &SelfUpdateCommand{
			Command: "brew",
			Args:    []string{"upgrade", "stell"},
			Display: "brew upgrade stell",
		}, nil
	default:
		return nil, fmt.Errorf("self-update unavailable for install method %q; try: go install %s", method, spec)
	}
}

// UnavailableInstruction возвращает подсказку для ручного обновления.
func UnavailableInstruction(modulePath, ver string) string {
	if modulePath == "" {
		modulePath = version.DefaultModulePath
	}
	spec := modulePath + "@v" + strings.TrimPrefix(strings.TrimSpace(ver), "v")
	return fmt.Sprintf("Run manually: go install %s", spec)
}

// RunSelfUpdate выполняет команду self-update.
func RunSelfUpdate(ctx context.Context, cmd *SelfUpdateCommand) error {
	if cmd == nil {
		return fmt.Errorf("nil self-update command")
	}
	c := exec.CommandContext(ctx, cmd.Command, cmd.Args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = append(os.Environ(), "GOTOOLCHAIN=auto")
	if runtime.GOOS == "windows" {
		c.Env = append(c.Env, "GOTOOLCHAIN=auto")
	}
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmd.Display, err)
	}
	return nil
}
