package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"stell/coding-agent/internal/packages"
)

func runInstall(args []string) int {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	global := fs.Bool("global", false, "install to ~/.stell/agent/packages")
	workspace := fs.String("workspace", "", "workspace root")
	_ = fs.Parse(args)
	srcArgs := fs.Args()
	if len(srcArgs) == 0 {
		fmt.Fprintln(os.Stderr, "usage: stell install [--global] <local-path|git:url[@ref]>")
		return 2
	}

		app, err := bootstrap(*workspace, "", true, bootstrapOpts{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}

	scope := "project"
	if *global {
		scope = "global"
	}
	mgr := packages.NewManager(app.Config.GlobalDir, app.Config.ProjectDir, scope)
	rec, err := mgr.Install(context.Background(), srcArgs[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	fmt.Printf("installed %s@%s → %s\n", rec.Name, rec.Version, rec.InstallPath)
	return 0
}
