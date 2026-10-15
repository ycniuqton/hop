//go:build !windows

package ui

// prepare is a no-op where terminals understand ANSI escape codes natively.
func (terminal) prepare(any) bool { return true }
