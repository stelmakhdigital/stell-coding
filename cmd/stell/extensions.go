package main

import (
	"context"
	"path/filepath"

	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-coding/internal/extensions"
	"github.com/stelmakhdigital/stell-coding/internal/packages"
	"github.com/stelmakhdigital/stell-agent/tools"
)

type pkgLister struct {
	global  *packages.Manager
	project *packages.Manager
}

func (p *pkgLister) List() ([]packages.Record, error) {
	var out []packages.Record
	if p.global != nil {
		recs, err := p.global.List()
		if err != nil {
			return nil, err
		}
		out = append(out, recs...)
	}
	if p.project != nil {
		recs, err := p.project.List()
		if err != nil {
			return nil, err
		}
		out = append(out, recs...)
	}
	return out, nil
}

func bootstrapExtensions(cfg *config.Config, rt *tools.Runtime, ws, extraDir string, broker *extensions.GrantBroker, interactive bool) (*extensions.Supervisor, error) {
	lister := &pkgLister{
		global:  packages.NewManager(cfg.GlobalDir, cfg.ProjectDir, "global"),
		project: packages.NewManager(cfg.GlobalDir, cfg.ProjectDir, "project"),
	}
	sup := extensions.NewSupervisor(ws, lister, rt)
	sup.GrantBroker = broker
	settingsPath := filepath.Join(cfg.GlobalDir, "settings.json")
	if cfg.ProjectDir != "" {
		settingsPath = filepath.Join(cfg.ProjectDir, "settings.json")
	}
	sup.GrantChecker = extensions.NewBrokerGrantChecker(broker, &cfg.Settings, settingsPath)
	sup.Interactive = interactive
	sup.ExtraDirs = extensions.DiscoverLooseDirs(cfg.GlobalDir, cfg.ProjectDir, cfg.Settings.Extensions)
	if extraDir != "" {
		if abs, err := filepath.Abs(extraDir); err == nil {
			sup.ExtraDirs = append(sup.ExtraDirs, abs)
		}
	}
	if err := sup.Bootstrap(context.Background()); err != nil {
		return nil, err
	}
	return sup, nil
}
