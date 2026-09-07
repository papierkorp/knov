package logstest

import (
	"fmt"
	"os"
	"strings"
	"time"

	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/test"
)

// caseInMemoryRingBuffer covers handleAPIGetLogs' logging.GetRecentEntries - every LogError
// call records a ring buffer entry regardless of KNOV_LOG_LEVEL/KNOV_LOG_FILE_LEVEL filtering.
func caseInMemoryRingBuffer() test.CaseResult {
	name := "in-memory-ring-buffer"

	marker := newMarker()
	logging.LogError(logging.KeyInAppTests, probeNote+"%s ring buffer probe", marker)

	entries := logging.GetRecentEntries(500)
	found := false
	for _, e := range entries {
		if e.Key == logging.KeyInAppTests && strings.Contains(e.Message, marker) {
			found = true
			break
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("GetRecentEntries includes a %s-keyed entry containing %q", logging.KeyInAppTests, marker),
		Actual:   fmt.Sprintf("found=%v (%d recent entries)", found, len(entries)),
		Success:  found,
	}
	if !found {
		cr.Error = "logged message did not appear in the in-memory ring buffer"
	}
	return cr
}

// caseParseLogContinuation covers render.ParseLogLines: (1) the fixed RFC3339 machine
// timestamp on a real line round-trips into entry.Time regardless of the configured date/time
// display style, and (2) continuation folding - a log record spanning several file lines
// (stack traces, pretty-printed JSON) is written with only its first line carrying a timestamp
// prefix, so the follow-on lines must be folded into the previous entry's Message rather than
// dropped or turned into their own rows, while a continuation line with nothing before it is
// dropped without panicking.
func caseParseLogContinuation() test.CaseResult {
	name := "parse-log-continuation"

	if !logging.HasFileLogging() {
		return errCase(name, fmt.Errorf("file logging is disabled, cannot exercise log parsing"))
	}

	marker := newMarker()
	logging.LogError(logging.KeyInAppTests, probeNote+"%s L0\n%s L1\n%s L2", marker, marker, marker)

	data, err := os.ReadFile(logging.LogFilePath(logging.KeyInAppTests))
	if err != nil {
		return errCase(name, err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	markerEntries := func(in []string) []logging.LogEntry {
		var out []logging.LogEntry
		for _, e := range render.ParseLogLines(logging.KeyInAppTests, in) {
			if strings.Contains(e.Message, marker) {
				out = append(out, e)
			}
		}
		return out
	}

	folded := markerEntries(lines)
	foldedOK := len(folded) == 1 && strings.Contains(folded[0].Message, "\n") &&
		strings.Contains(folded[0].Message, "L0") &&
		strings.Contains(folded[0].Message, "L1") &&
		strings.Contains(folded[0].Message, "L2")

	// the fixed machine-format (RFC3339) timestamp on the first line must round-trip into
	// entry.Time no matter what date/time display style is configured
	tsOK := len(folded) == 1 && !folded[0].Time.IsZero() && time.Since(folded[0].Time) < 5*time.Minute

	// a continuation line with nothing before it is dropped, not folded or panicked on
	orphan := markerEntries(append([]string{"    " + marker + " orphan"}, lines...))
	orphanOK := len(orphan) == 1 && !strings.Contains(orphan[0].Message, "orphan")

	success := foldedOK && tsOK && orphanOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "the 3-line record parses to one entry whose Message keeps L0/L1/L2 and whose Time round-trips; a leading orphan continuation is dropped",
		Actual:   fmt.Sprintf("foldedOK=%v tsOK=%v orphanOK=%v (marker entries: %d)", foldedOK, tsOK, orphanOK, len(folded)),
		Success:  success,
	}
	if !success {
		cr.Error = "ParseLogLines did not fold multi-line records as expected"
	}
	return cr
}

// caseDownloadPathGuard covers handleAPIDownloadLogs' path-safety guard (a name containing a
// path separator is rejected) and confirms a valid name resolves to the real log file with its
// raw, untouched content - the download handler itself does nothing but io.Copy the file.
// Uses KeyApp rather than KeyInAppTests: this checks that resolveDownloadPath (GetLogsDir()-
// based, same as the real handler) agrees with where the file actually is, which only holds
// for keys that aren't exempt from SetIsolatedLogsDir's redirect - KeyInAppTests is, KeyApp
// isn't.
func caseDownloadPathGuard() test.CaseResult {
	name := "download-path-guard"

	marker := newMarker()
	logging.LogError(logging.KeyApp, probeNote+"%s download probe", marker)

	rejected := resolveDownloadPath("sub/evil.log") == ""

	resolved := resolveDownloadPath(logging.KeyApp.String() + ".log")
	expectedPath := logging.LogFilePath(logging.KeyApp)
	pathOK := resolved != "" && resolved == expectedPath

	contentOK := false
	if resolved != "" {
		data, err := os.ReadFile(resolved)
		if err != nil {
			return errCase(name, err)
		}
		contentOK = strings.Contains(string(data), marker)
	}

	success := rejected && pathOK && contentOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "path-separator name rejected, valid name resolves to the real log file containing the probe untouched",
		Actual:   fmt.Sprintf("rejected=%v pathOK=%v contentOK=%v", rejected, pathOK, contentOK),
		Success:  success,
	}
	if !success {
		cr.Error = "download path resolution/content did not behave as expected"
	}
	return cr
}
