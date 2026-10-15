//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

// detach starts the refresh in its own session so the shell never waits for it.
func (processSpawner) detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
