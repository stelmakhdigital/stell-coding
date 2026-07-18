package packages

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func gitClone(ctx context.Context, url, ref, dest string) error {
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, dest)
	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
