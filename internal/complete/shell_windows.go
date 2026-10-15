//go:build windows

package complete

// defaultShell is PowerShell, because Windows does not set $SHELL.
const defaultShell Shell = PowerShell
