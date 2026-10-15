//go:build !windows

package complete

// defaultShell is empty: on Unix the shell always comes from $SHELL.
const defaultShell Shell = ""
