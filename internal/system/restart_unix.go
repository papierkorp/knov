//go:build !windows

package system

import (
	"os"
	"syscall"
)

// Restart replaces the current process image in place via exec, keeping the same PID - so a
// supervisor tracking that PID (systemd, Docker, launchd, ...) never sees the process exit,
// and it comes back up standalone even with no supervisor at all. On success this call never
// returns.
func Restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(exe, os.Args, os.Environ())
}
