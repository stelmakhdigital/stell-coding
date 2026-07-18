package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runShareHF(args []string) int {
	fs := flag.NewFlagSet("share-hf", flag.ContinueOnError)
	session := fs.String("session", "", "path to session .jsonl")
	repo := fs.String("repo", "", "HF dataset repo (user/name)")
	private := fs.Bool("private", false, "create/upload as private dataset")
	_ = fs.Parse(args)
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "usage: stell share-hf --repo user/dataset [--session path.jsonl] [--private]")
		return 2
	}
	path := *session
	if path == "" {
		fmt.Fprintln(os.Stderr, "stell share-hf: --session required (path to .jsonl)")
		return 2
	}
	script := filepath.Join(findRepoRoot(), "scripts", "share-hf.sh")
	if _, err := os.Stat(script); err != nil {
		// Запасной вариант: вызов huggingface-cli напрямую
		return shareHFDirect(path, *repo, *private)
	}
	cmdArgs := []string{script, path, *repo}
	if *private {
		cmdArgs = append(cmdArgs, "true")
	}
	cmd := exec.Command("bash", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func shareHFDirect(session, repo string, private bool) int {
	bin := "huggingface-cli"
	if _, err := exec.LookPath(bin); err != nil {
		bin = "hf"
	}
	args := []string{"upload", repo, session, "--repo-type", "dataset"}
	if private {
		args = append(args, "--private")
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func findRepoRoot() string {
	wd, _ := os.Getwd()
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "scripts", "share-hf.sh")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return wd
}
