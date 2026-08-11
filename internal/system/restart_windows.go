//go:build windows

package system

import (
	"os"
	"os/exec"
)

// Restart spawns a new instance of the running binary and returns once it has started -
// Windows has no in-process exec, so the caller must exit the old process itself afterwards.
func Restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return cmd.Start()
}
