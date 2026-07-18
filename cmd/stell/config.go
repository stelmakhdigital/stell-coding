package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stelmakhdigital/stell-coding/internal/config"
)

func runConfig(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: stell config list|get|set <key> [value]")
		return 2
	}
	switch args[0] {
	case "list":
		return configList()
	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: stell config get <key>")
			return 2
		}
		return configGet(args[1])
	case "set":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: stell config set <key> <value>")
			return 2
		}
		return configSet(args[1], args[2])
	default:
		fmt.Fprintln(os.Stderr, "unknown config command:", args[0])
		return 2
	}
}

func settingsPath() (string, error) {
	dir, err := config.GlobalDir()
	if err != nil {
		return "", err
	}
	if p := os.Getenv("STELL_CONFIG"); p != "" {
		return p, nil
	}
	return filepath.Join(dir, "settings.json"), nil
}

func configList() int {
	path, err := settingsPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("{}")
			return 0
		}
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	fmt.Println(string(data))
	return 0
}

func configGet(key string) int {
	path, err := settingsPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	v, ok := m[key]
	if !ok {
		fmt.Fprintln(os.Stderr, "stell: key not found:", key)
		return 1
	}
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
	return 0
}

func configSet(key, value string) int {
	path, err := settingsPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	m := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	var v any
	if err := json.Unmarshal([]byte(value), &v); err != nil {
		v = value
	}
	m[key] = v
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "stell:", err)
		return 1
	}
	return 0
}

func runPkg(args []string) int {
	fs := flag.NewFlagSet("pkg", flag.ExitOnError)
	global := fs.Bool("global", false, "use global package store")
	workspace := fs.String("workspace", "", "workspace root")
	_ = fs.Parse(args)
	sub := fs.Args()
	if len(sub) == 0 {
		fmt.Fprintln(os.Stderr, "usage: stell pkg list|remove|update [name]")
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
	mgr := newPkgManager(app, scope)

	switch sub[0] {
	case "list":
		recs, err := mgr.List()
		if err != nil {
			fmt.Fprintln(os.Stderr, "stell:", err)
			return 1
		}
		for _, r := range recs {
			fmt.Printf("%s@%s  %s  %s\n", r.Name, r.Version, r.Source, r.InstallPath)
		}
		return 0
	case "remove":
		if len(sub) < 2 {
			fmt.Fprintln(os.Stderr, "usage: stell pkg remove <name>")
			return 2
		}
		if err := mgr.Remove(sub[1]); err != nil {
			fmt.Fprintln(os.Stderr, "stell:", err)
			return 1
		}
		fmt.Println("removed", sub[1])
		return 0
	case "update":
		name := ""
		if len(sub) >= 2 {
			name = sub[1]
		}
		if err := mgr.Update(context.Background(), name); err != nil {
			fmt.Fprintln(os.Stderr, "stell:", err)
			return 1
		}
		fmt.Println("update complete")
		return 0
	default:
		fmt.Fprintln(os.Stderr, "unknown pkg command:", sub[0])
		return 2
	}
}
