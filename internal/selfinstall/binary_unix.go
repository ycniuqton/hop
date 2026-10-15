//go:build !windows

package selfinstall

import "os"

// reinstallHint is shown when hop cannot write over its own binary.
const reinstallHint = "re-run with sudo, or reinstall with the curl installer"

// swap renames the new binary over the old one; Unix allows this while the old one runs.
func (b Binary) swap(newPath string) error {
	return os.Rename(newPath, b.Path)
}

// Delete removes the binary; a running program keeps its open copy until it exits.
func (b Binary) Delete() error {
	return os.Remove(b.Path)
}
