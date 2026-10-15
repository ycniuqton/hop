//go:build windows

package cli

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// detach starts the refresh without a console in its own process group, so the shell never waits.
func (processSpawner) detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS, HideWindow: true}
}
