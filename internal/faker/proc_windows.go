//go:build windows

package faker

import (
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// detachedAttr opens the fake process in its own console window.
func detachedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
}

// desktopDir resolves the real Desktop folder, which may be redirected (e.g. to OneDrive).
func desktopDir() string {
	if p, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, 0); err == nil && p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}
