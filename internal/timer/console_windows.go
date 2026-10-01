//go:build windows

package timer

import (
	"os"

	"golang.org/x/sys/windows"
)

// openConsole opens the console window attached to this process directly.
// os/exec connects a child's standard handles to NUL when none are given, so a
// fake started from the main UI cannot rely on stdin/stdout.
func openConsole() (in, out *os.File, err error) {
	out, err = os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	in, err = os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		out.Close()
		return nil, nil, err
	}

	h := windows.Handle(out.Fd())
	var mode uint32
	if windows.GetConsoleMode(h, &mode) == nil {
		_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
	return in, out, nil
}
