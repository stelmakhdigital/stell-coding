package discovery

import (
	"stell/coding-agent/internal/catalog"
	"stell/coding-agent/internal/config"
	"stell/coding-agent/internal/packages"
	"stell/coding-agent/internal/prompts"
	"stell/coding-agent/internal/skills"
)

type Catalog struct {
	Skills  *skills.Registry
	Prompts *prompts.Registry
}

func Load(cfg *config.Config) (*Catalog, error) {
	pkgMgr := packages.NewManager(cfg.GlobalDir, cfg.ProjectDir, "project")
	globalPkg := packages.NewManager(cfg.GlobalDir, cfg.ProjectDir, "global")

	pkgSkillDirs, _ := pkgMgr.EnabledResourceDirs("skills")
	pkgSkillDirs = append(pkgSkillDirs, mustDirs(globalPkg, "skills")...)
	pkgPromptDirs, _ := pkgMgr.EnabledResourceDirs("prompts")
	pkgPromptDirs = append(pkgPromptDirs, mustDirs(globalPkg, "prompts")...)

	skillDirs := catalog.ResourceDirsWithPackages(
		cfg.GlobalDir, cfg.ProjectDir, pkgSkillDirs, cfg.Settings.Skills, "skills",
	)
	skillDirs = append(skillDirs, catalog.AgentsSkillsDirs(cfg.Workspace)...)

	promptDirs := catalog.ResourceDirsWithPackages(
		cfg.GlobalDir, cfg.ProjectDir, pkgPromptDirs, cfg.Settings.Prompts, "prompts",
	)

	sk := skills.NewRegistry()
	sk.Scan(skillDirs)

	pr := prompts.NewRegistry()
	pr.Scan(promptDirs)

	return &Catalog{Skills: sk, Prompts: pr}, nil
}

func mustDirs(m *packages.Manager, kind string) []string {
	d, err := m.EnabledResourceDirs(kind)
	if err != nil {
		return nil
	}
	return d
}
