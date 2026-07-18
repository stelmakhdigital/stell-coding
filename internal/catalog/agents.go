package catalog

import (
	"os"
	"path/filepath"
)

// AgentsSkillsDirs возвращает каталоги .agents/skills от workspace до
// корня git-репозитория (семантика stell).
func AgentsSkillsDirs(workspace string) []Dir {
	gitRoot := findGitRoot(workspace)
	var dirs []Dir
	dir := filepath.Clean(workspace)
	for {
		p := filepath.Join(dir, ".agents", "skills")
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			dirs = append(dirs, Dir{Path: p, Source: SourceAgents})
		}
		if dir == gitRoot {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

func findGitRoot(start string) string {
	dir := filepath.Clean(start)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}
