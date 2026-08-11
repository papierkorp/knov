// Package system holds app-lifecycle operations (restart, ...) - the same domain as the
// server's /api/system routes.
package system

import "os"

// CanRestart reports whether Restart is likely to succeed, without actually restarting - e.g.
// it fails under `go run`, which builds to a throwaway temp binary that isn't guaranteed to
// still be there. Callers should check this before committing to a response, since a
// successful Restart on Linux/macOS never returns.
func CanRestart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	_, err = os.Stat(exe)
	return err
}
