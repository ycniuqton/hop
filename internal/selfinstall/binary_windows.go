//go:build windows

package selfinstall

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// Cleaner describes the helper that deletes a moved-aside binary after hop exits.
var Cleaner = struct{ Shell, ShellEnv, Script string }{
	Shell: "cmd.exe", ShellEnv: "ComSpec", Script: `/c "ping -n 3 127.0.0.1 >NUL & del /f /q "%s""`,
}

// reinstallHint is shown when hop cannot write over its own binary.
const reinstallHint = "run the terminal as Administrator, or reinstall with install.ps1"

// swap moves the running hop.exe aside (Windows allows renaming, not overwriting) and puts the
// new one in its place, restoring the old file if the second step fails.
func (b Binary) swap(newPath string) error {
	old := b.Path + StaleSuffix
	_ = os.Remove(old)
	if err := os.Rename(b.Path, old); err != nil {
		return err
	}
	if err := os.Rename(newPath, b.Path); err != nil {
		return errors.Join(err, os.Rename(old, b.Path))
	}
	return nil
}

// Delete moves hop.exe aside and starts a detached helper that deletes it once hop has exited.
func (b Binary) Delete() error {
	old := b.Path + StaleSuffix
	_ = os.Remove(old)
	if err := os.Rename(b.Path, old); err != nil {
		return err
	}
	shell := os.Getenv(Cleaner.ShellEnv)
	if shell == "" {
		shell = Cleaner.Shell
	}
	cmd := exec.Command(shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       shell + " " + fmt.Sprintf(Cleaner.Script, old),
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
		HideWindow:    true,
	}
	if cmd.Start() == nil {
		_ = cmd.Process.Release()
	}
	return nil
}
