//go:build !unix

package extensions

import "os/exec"

func configureProcessGroup(cmd *exec.Cmd) {}

func terminateProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
