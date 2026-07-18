package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-coding/internal/telemetry"
	"github.com/stelmakhdigital/stell-coding/internal/update"
	"github.com/stelmakhdigital/stell-coding/internal/version"
)

type updateTargetKind int

const (
	targetSelf updateTargetKind = iota
	targetExtensions
	targetAll
)

type updateOptions struct {
	target              updateTargetKind
	extensionSource     string
	force               bool
	global              bool
	workspace           string
	showExtensionsHint  bool
}

func runUpdate(args []string) int {
	if update.Offline() {
		fmt.Fprintln(os.Stderr, "stell: offline mode — updates disabled")
		return 1
	}
	opts, err := parseUpdateArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 2
	}
	if opts.showExtensionsHint {
		fmt.Fprintln(os.Stderr, "hint: use --extensions or --all to update installed packages")
	}

	ctx := context.Background()
	if opts.target == targetExtensions || opts.target == targetAll {
		if err := runExtensionsUpdate(ctx, opts); err != nil {
			fmt.Fprintln(os.Stderr, "stell:", err)
			return 1
		}
	}
	if opts.target == targetSelf || opts.target == targetAll {
		if err := runSelfUpdate(ctx, opts.force); err != nil {
			fmt.Fprintln(os.Stderr, "stell:", err)
			return 1
		}
	}
	return 0
}

func parseUpdateArgs(args []string) (updateOptions, error) {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	self := fs.Bool("self", false, "update stell only")
	extensions := fs.Bool("extensions", false, "update packages only")
	all := fs.Bool("all", false, "update stell and packages")
	extension := fs.String("extension", "", "update one package source")
	force := fs.Bool("force", false, "reinstall stell even if current")
	global := fs.Bool("global", false, "package scope: global")
	workspace := fs.String("workspace", "", "workspace root")
	if err := fs.Parse(args); err != nil {
		return updateOptions{}, err
	}
	pos := fs.Args()
	opts := updateOptions{force: *force, global: *global, workspace: *workspace}

	if *all && (*self || *extensions || *extension != "" || len(pos) > 0) {
		return updateOptions{}, fmt.Errorf("--all cannot be combined with other update targets")
	}
	if *extension != "" && (*self || *extensions || *all || len(pos) > 0) {
		return updateOptions{}, fmt.Errorf("--extension cannot be combined with other update targets")
	}
	if *self && (*extensions || *all) {
		return updateOptions{}, fmt.Errorf("--self cannot be combined with --extensions or --all")
	}
	if *extensions && *all {
		return updateOptions{}, fmt.Errorf("--extensions cannot be combined with --all")
	}

	switch {
	case *all:
		opts.target = targetAll
	case *extensions:
		opts.target = targetExtensions
	case *self:
		opts.target = targetSelf
	case *extension != "":
		opts.target = targetExtensions
		opts.extensionSource = *extension
	default:
		if len(pos) > 0 {
			switch strings.ToLower(pos[0]) {
			case "self", "stell":
				opts.target = targetSelf
				pos = pos[1:]
			default:
				opts.target = targetExtensions
				opts.extensionSource = pos[0]
				pos = pos[1:]
			}
		} else {
			opts.target = targetSelf
			opts.showExtensionsHint = true
		}
	}
	if len(pos) > 0 {
		return updateOptions{}, fmt.Errorf("unexpected arguments: %s", strings.Join(pos, " "))
	}
	return opts, nil
}

func runExtensionsUpdate(ctx context.Context, opts updateOptions) error {
	app, err := bootstrap(opts.workspace, "", true, bootstrapOpts{})
	if err != nil {
		return err
	}
	scope := "project"
	if opts.global {
		scope = "global"
	}
	mgr := newPkgManager(app, scope)
	if opts.extensionSource != "" {
		recs, err := mgr.List()
		if err != nil {
			return err
		}
		var name string
		for _, r := range recs {
			if r.Source == opts.extensionSource || r.Name == opts.extensionSource {
				name = r.Name
				break
			}
		}
		if name == "" {
			return fmt.Errorf("package %q not installed", opts.extensionSource)
		}
		if err := mgr.Update(ctx, name); err != nil {
			return err
		}
		fmt.Println("updated", name)
		return nil
	}
	if err := mgr.Update(ctx, ""); err != nil {
		return err
	}
	fmt.Println("package update complete")
	return nil
}

func runSelfUpdate(ctx context.Context, force bool) error {
	release, err := update.GetLatestRelease(ctx, version.Version)
	if err != nil {
		return err
	}
	if release == nil {
		return fmt.Errorf("could not determine latest version")
	}
	modulePath := release.ModulePath
	if modulePath == "" {
		modulePath = version.DefaultModulePath
	}
	shouldRun := force || update.IsNewer(release.Version, version.Version)
	if !shouldRun {
		fmt.Printf("stell is already up to date (v%s)\n", version.Version)
		return nil
	}
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return err
	}
	method := update.DetectInstallMethod(execPath)
	cmd, err := update.GetSelfUpdateCommand(method, modulePath, release.Version)
	if err != nil {
		fmt.Fprintln(os.Stderr, update.UnavailableInstruction(modulePath, release.Version))
		return err
	}
	fmt.Printf("Updating stell from v%s to v%s...\n", version.Version, release.Version)
	if err := update.RunSelfUpdate(ctx, cmd); err != nil {
		return err
	}
	fmt.Printf("Updated stell from v%s to v%s\n", version.Version, release.Version)
	if note := strings.TrimSpace(release.Note); note != "" {
		fmt.Println(note)
	}
	reportSelfUpdateTelemetry(ctx)
	return nil
}

func reportSelfUpdateTelemetry(ctx context.Context) {
	path, err := settingsPath()
	if err != nil {
		return
	}
	var s config.Settings
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	if !telemetry.IsInstallTelemetryEnabled(s) {
		return
	}
	_ = telemetry.ReportInstall(ctx, version.Version, s.TrackingID)
}

func printVersion() {
	fmt.Println(version.Display())
}

// hasOfflineFlag проверяет сырые аргументы до разбора флагов.
func hasOfflineFlag(args []string) bool {
	for _, a := range args {
		if a == "--offline" {
			return true
		}
	}
	return false
}

func hasVersionFlag(args []string) bool {
	for _, a := range args {
		if a == "--version" || a == "-v" {
			return true
		}
	}
	return false
}

func initOfflineFromArgs(args []string) {
	if hasOfflineFlag(args) {
		update.EnableOffline()
	}
}
