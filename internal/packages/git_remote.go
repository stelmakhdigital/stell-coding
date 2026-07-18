package packages

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const gitNetworkTimeout = 10 * time.Second

var sha40 = regexp.MustCompile(`^[0-9a-f]{40}$`)

func gitHasAvailableUpdate(ctx context.Context, installedPath string) (bool, error) {
	if installedPath == "" {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, gitNetworkTimeout)
	defer cancel()

	localHead, err := gitOutput(ctx, installedPath, "rev-parse", "HEAD")
	if err != nil {
		return false, err
	}
	localHead = strings.TrimSpace(localHead)
	if !sha40.MatchString(localHead) {
		return false, nil
	}

	remoteHead, err := getRemoteGitHead(ctx, installedPath)
	if err != nil {
		return false, err
	}
	remoteHead = strings.TrimSpace(remoteHead)
	if !sha40.MatchString(remoteHead) {
		return false, nil
	}
	return localHead != remoteHead, nil
}

func getRemoteGitHead(ctx context.Context, installedPath string) (string, error) {
	upstream, err := gitOutput(ctx, installedPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	upstream = strings.TrimSpace(upstream)
	if upstream == "" || upstream == "HEAD" {
		upstream = "HEAD"
	}
	out, err := gitOutput(ctx, installedPath, "ls-remote", "origin", upstream)
	if err != nil {
		out, err = gitOutput(ctx, installedPath, "ls-remote", "origin", "HEAD")
		if err != nil {
			return "", err
		}
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && sha40.MatchString(fields[0]) {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no remote head for %s", installedPath)
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("git %v: %w: %s", args, err, msg)
		}
		return "", fmt.Errorf("git %v: %w", args, err)
	}
	return string(out), nil
}
