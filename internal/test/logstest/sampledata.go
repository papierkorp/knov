// Package logstest - shared helpers
package logstest

import (
	"fmt"
	"os"
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

// inAppTestsLogPath is the file logging.KeyInAppTests writes to - already a real, shared log
// key (the job scheduler logs every suite run's pass/fail summary here), reused directly
// rather than inventing a synthetic key.
func inAppTestsLogPath() string {
	return filepath.Join(logging.GetLogsDir(), logging.KeyInAppTests.String()+".log")
}

// readLines reads a log file's current lines, oldest first - same shape handleAPIGetLogsFile's
// bufio.Scanner loop produces.
func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := strings.TrimRight(string(data), "\n")
	if content == "" {
		return nil, nil
	}
	return strings.Split(content, "\n"), nil
}

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
