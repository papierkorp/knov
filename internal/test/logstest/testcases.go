package logstest

import (
	"fmt"
	"os"
	"strings"

	"knov/internal/logging"
	"knov/internal/test"
)

// caseInMemoryRingBuffer covers handleAPIGetLogs' logging.GetRecentEntries - every LogError
// call records a ring buffer entry regardless of KNOV_LOG_LEVEL/KNOV_LOG_FILE_LEVEL filtering.
func caseInMemoryRingBuffer() test.CaseResult {
	name := "in-memory-ring-buffer"

	marker := newMarker()
	logging.LogError(logging.KeyInAppTests, "%s ring buffer probe", marker)

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

// caseFilePaginationChunking replicates handleAPIGetLogsFile's offset/limit slicing
// (internal/server/api_system.go:163-179) directly - that arithmetic is inline handler logic
// with no exported wrapper. Writes 6 known probe lines, then pages them 3-at-a-time: the first
// page (offset=0) must be the latest 3 lines with hasMore=true (total 6 > limit 3), the second
// page (offset=3) must be the earlier 3 lines - the same "load earlier lines" chunking the
// admin log-file view uses.
func caseFilePaginationChunking() test.CaseResult {
	name := "file-pagination-chunking"

	if !logging.HasFileLogging() {
		return errCase(name, fmt.Errorf("file logging is disabled, cannot exercise file pagination"))
	}

	marker := newMarker()
	const n = 6
	for i := 0; i < n; i++ {
		logging.LogError(logging.KeyInAppTests, "%s line %d", marker, i)
	}

	lines, err := readLines(inAppTestsLogPath())
	if err != nil {
		return errCase(name, err)
	}
	total := len(lines)

	page := func(limit, offset int) (chunk []string, hasMore bool) {
		end := total - offset
		if end < 0 {
			end = 0
		}
		start := end - limit
		if start < 0 {
			start = 0
		}
		return lines[start:end], start > 0
	}

	latest, latestHasMore := page(3, 0)
	earlier, _ := page(3, 3)

	latestOK := linesContainMarkerRange(latest, marker, 3, 5)
	earlierOK := linesContainMarkerRange(earlier, marker, 0, 2)

	success := latestOK && earlierOK && latestHasMore
	cr := test.CaseResult{
		Name:     name,
		Expected: "offset=0/limit=3 returns lines 3-5 with hasMore=true, offset=3/limit=3 returns lines 0-2",
		Actual:   fmt.Sprintf("latestOK=%v earlierOK=%v latestHasMore=%v (total=%d)", latestOK, earlierOK, latestHasMore, total),
		Success:  success,
	}
	if !success {
		cr.Error = "file pagination/chunking did not slice the expected lines"
	}
	return cr
}

// linesContainMarkerRange checks every "<marker> line <i>" probe for i in [from, to] appears
// somewhere in chunk - substring containment rather than strict positional equality, tolerating
// any real log activity from the running app interleaved with the probe lines.
func linesContainMarkerRange(chunk []string, marker string, from, to int) bool {
	joined := strings.Join(chunk, "\n")
	for i := from; i <= to; i++ {
		if !strings.Contains(joined, fmt.Sprintf("%s line %d", marker, i)) {
			return false
		}
	}
	return true
}

// caseDownloadPathGuard covers handleAPIDownloadLogs' path-safety guard (a name containing a
// path separator is rejected) and confirms a valid name resolves to the real log file with its
// raw, untouched content - the download handler itself does nothing but io.Copy the file.
func caseDownloadPathGuard() test.CaseResult {
	name := "download-path-guard"

	marker := newMarker()
	logging.LogError(logging.KeyInAppTests, "%s download probe", marker)

	rejected := resolveDownloadPath("sub/evil.log") == ""

	resolved := resolveDownloadPath(logging.KeyInAppTests.String() + ".log")
	expectedPath := inAppTestsLogPath()
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
