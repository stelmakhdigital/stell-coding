package themes

import (
	"os"
	"path/filepath"
	"strings"

	"stell/coding-agent/internal/packages"
	"stell/coding-agent/internal/trust"
)

// ResolveOpts настраивает discovery тем (PackageManager + ResourceLoader).
type ResolveOpts struct {
	GlobalDir        string
	ProjectDir       string
	Workspace        string   // for trust check; defaults to ProjectDir
	ExtraPaths       []string // settings.themes[] files or directories
	PackageThemeDirs []string // if nil, scanned from installed packages
	Trusted          *bool    // nil → load trust.json
}

// Resolve находит темы с приоритетом: project > settings.extra > user > package > builtin.
// Первая регистрация имени побеждает (источники с более высоким приоритетом сканируются первыми).
func Resolve(opts ResolveOpts) ([]Theme, error) {
	seen := map[string]bool{}
	var out []Theme
	add := func(list []Theme) {
		for _, t := range list {
			name := strings.TrimSpace(t.Name)
			if name == "" || seen[name] {
				continue
			}
			if err := t.Validate(); err != nil {
				continue
			}
			seen[name] = true
			out = append(out, t)
		}
	}

	trusted := opts.Trusted != nil && *opts.Trusted
	if opts.Trusted == nil {
		ws := opts.Workspace
		if ws == "" {
			ws = opts.ProjectDir
		}
		if tf, err := trust.LoadMerged(opts.GlobalDir, ws); err == nil && tf != nil {
			trusted = tf.IsTrusted(ws)
		}
	}

	projectRoot := opts.ProjectDir
	if projectRoot == "" {
		projectRoot = opts.Workspace
	}

	// 1. Project auto-темы (только trusted): {project}/.stell/themes или {project}/themes
	if trusted && projectRoot != "" {
		for _, d := range []string{
			filepath.Join(projectRoot, ".stell", "themes"),
			filepath.Join(projectRoot, "themes"),
		} {
			if list, err := LoadDir(d); err == nil {
				add(list)
			}
		}
	}

	// 2. settings.themes[] (файлы или каталоги)
	for _, p := range opts.ExtraPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			if list, err := LoadDir(p); err == nil {
				add(list)
			}
			continue
		}
		if t, err := Load(p); err == nil {
			add([]Theme{*t})
		}
	}

	// 3. Global user-темы
	if opts.GlobalDir != "" {
		if list, err := LoadDir(filepath.Join(opts.GlobalDir, "themes")); err == nil {
			add(list)
		}
	}

	// 4. Каталоги тем пакетов
	pkgDirs := opts.PackageThemeDirs
	if pkgDirs == nil {
		pkgDirs = PackageThemeDirs(opts.GlobalDir, projectRoot)
	}
	for _, d := range pkgDirs {
		if list, err := LoadDir(d); err == nil {
			add(list)
		}
	}

	// 5. Built-in в конце (низший приоритет)
	add([]Theme{DarkTheme(), LightTheme()})
	return out, nil
}

// PackageThemeDirs возвращает каталоги ресурсов тем из установленных пакетов (global + project).
func PackageThemeDirs(globalDir, projectDir string) []string {
	var out []string
	seen := map[string]bool{}
	scanRoot := func(root string) {
		if root == "" {
			return
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "." || e.Name() == ".." {
				continue
			}
			dir := filepath.Join(root, e.Name())
			m, err := packages.LoadManifest(dir)
			if err != nil {
				continue
			}
			for _, d := range m.ResourceDirs(dir, "themes") {
				if !seen[d] {
					seen[d] = true
					out = append(out, d)
				}
			}
		}
	}
	if globalDir != "" {
		scanRoot(filepath.Join(globalDir, "packages"))
	}
	if projectDir != "" {
		scanRoot(filepath.Join(projectDir, ".stell", "packages"))
	}
	return out
}

// FindByName возвращает тему с данным именем из Resolve или nil.
func FindByName(opts ResolveOpts, name string) *Theme {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	// Auto-setting "light/dark" — не одно имя темы.
	if strings.Contains(name, "/") {
		return nil
	}
	list, err := Resolve(opts)
	if err != nil {
		return nil
	}
	for i := range list {
		if list[i].Name == name {
			t := list[i]
			return &t
		}
	}
	// Built-in по имени, даже если Validate как-то отфильтровал.
	switch name {
	case "dark":
		t := DarkTheme()
		return &t
	case "light":
		t := LightTheme()
		return &t
	}
	return nil
}

// Discover оставлен для вызывающих; использует Resolve со сканом пакетов.
func Discover(globalDir, projectDir string, packageThemeDirs []string, trusted bool) ([]Theme, error) {
	t := trusted
	return Resolve(ResolveOpts{
		GlobalDir:        globalDir,
		ProjectDir:       projectDir,
		PackageThemeDirs: packageThemeDirs,
		Trusted:          &t,
	})
}
