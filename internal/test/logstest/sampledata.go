// Package logstest - shared helpers
package logstest

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/logging"
	"knov/internal/test"
)

// newMarker returns a unique probe string so a case's own log lines can be told apart from
// real, concurrently-written app log activity sharing the same file.
func newMarker() string {
	return fmt.Sprintf("logstest-probe-%d", time.Now().UnixNano())
}

// probeNote prefixes every deliberate error/warning-level log line this suite writes, so
// anyone skimming logs/in-app-tests.log (or app.log for caseDownloadPathGuard) can tell at a
// glance that an "error" entry is an expected test probe rather than a real failure.
const probeNote = "deliberate test probe, not a real error - "

// resolveDownloadPath replicates handleAPIDownloadLogs/handleAPIGetLogsFile's unexported
// resolveLogFilePath path-safety rule (internal/server/api_system.go) for a named (non-default)
// log file: reject any name containing a path separator, then require the joined path to stay
// under the logs directory.
func resolveDownloadPath(name string) string {
	if strings.ContainsAny(name, "/\\") {
		return ""
	}
	dir := logging.GetLogsDir()
	p := filepath.Join(dir, name)
	if !strings.HasPrefix(filepath.Clean(p), filepath.Clean(dir)) {
		return ""
	}
	return p
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
