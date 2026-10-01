//go:build !windows

package timer

import "os"

// openConsole opens the controlling terminal directly.
func openConsole() (in, out *os.File, err error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	return tty, tty, nil
}
