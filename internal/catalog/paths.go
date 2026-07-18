package catalog

import (
	"os"
	"path/filepath"
)

type Source string

const (
	SourceGlobal  Source = "global"
	SourcePackage Source = "package"
	SourceProject Source = "project"
	SourceExtra   Source = "extra"
	SourceAgents  Source = "agents"
)

type Dir struct {
	Path   string
	Source Source
}

func ResourceDirs(globalDir, projectDir string, extra []string, subdir string) []Dir {
	return ResourceDirsWithPackages(globalDir, projectDir, nil, extra, subdir)
}

func ResourceDirsWithPackages(globalDir, projectDir string, packageDirs, extra []string, subdir string) []Dir {
	var dirs []Dir
	add := func(root string, src Source) {
		if root == "" {
			return
		}
		p := filepath.Join(root, subdir)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			dirs = append(dirs, Dir{Path: p, Source: src})
		}
	}
	addDir := func(path string, src Source) {
		if path == "" {
			return
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			dirs = append(dirs, Dir{Path: path, Source: src})
		}
	}
	add(globalDir, SourceGlobal)
	for _, p := range packageDirs {
		addDir(p, SourcePackage)
	}
	for _, e := range extra {
		addDir(e, SourceExtra)
	}
	if projectDir != "" {
		add(projectDir, SourceProject)
	}
	return dirs
}

func AgentsSkillsDir(workspace string) string {
	dirs := AgentsSkillsDirs(workspace)
	if len(dirs) == 0 {
		return ""
	}
	return dirs[0].Path
}
