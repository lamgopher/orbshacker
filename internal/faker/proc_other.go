//go:build !windows

package faker

import (
	"os"
	"path/filepath"
	"syscall"
)

func detachedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

func desktopDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}
