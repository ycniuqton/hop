//go:build windows

package ui

import (
	"os"

	"golang.org/x/sys/windows"
)

// prepare turns on ANSI escape handling for a console stream; false means colours and the
// arrow-key menu must not be used on it.
func (terminal) prepare(v any) bool {
	f, ok := v.(*os.File)
	if !ok {
		return false
	}
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	return windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
