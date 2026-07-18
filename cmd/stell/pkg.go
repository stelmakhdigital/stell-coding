package main

import (
	"github.com/stelmakhdigital/stell-coding/internal/packages"
)

func newPkgManager(app *App, scope string) *packages.Manager {
	return packages.NewManager(app.Config.GlobalDir, app.Config.ProjectDir, scope)
}
