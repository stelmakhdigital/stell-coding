package main

import (
	"stell/coding-agent/internal/packages"
)

func newPkgManager(app *App, scope string) *packages.Manager {
	return packages.NewManager(app.Config.GlobalDir, app.Config.ProjectDir, scope)
}
