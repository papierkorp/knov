// Package server ..
package server

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/job"
	"knov/internal/logging"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Invalidate cache
// @Description Removes all cache entries, forcing a rebuild on next access
// @Tags system
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Success 200 {string} string "cache invalidated"
// @Failure 500 {string} string "failed to invalidate cache"
// @Router /api/system/cache [delete]
func handleAPIInvalidateCache(w http.ResponseWriter, r *http.Request) {
	if err := job.RunCacheInvalidate(); err != nil {
		logging.LogError(logging.KeyApp, "failed to invalidate cache: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to invalidate cache"))
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "cache invalidated"))
	writeResponse(w, r, map[string]string{"status": "cache invalidated"}, "")
}

// @Summary Get recent log entries
// @Description Returns the most recent in-memory log entries across every key, newest first, as an HTML table (default) or - with raw=true - verbatim-style monospace lines. Powers the "Live" view on the admin logs page.
// @Tags system
// @Produce json,html
// @Param raw query bool false "render as verbatim monospace lines instead of a table"
// @Success 200 {string} string "log HTML"
// @Router /api/logs [get]
func handleAPIGetLogs(w http.ResponseWriter, r *http.Request) {
	entries := logging.GetRecentEntries(200)

	if r.URL.Query().Get("raw") == "true" {
		writeResponse(w, r, entries, render.RawLogLines(entries))
		return
	}
	writeResponse(w, r, entries, render.RenderLogTable(entries))
}

// parseUnixTimeParam parses a from/to query param carrying unix seconds
// (chosen over a formatted string so the server needn't guess which date
// display format the client used).
func parseUnixTimeParam(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(n, 0), true
}

// readLogFileLines reads every line of a log file, raising the scanner's token
// limit so long lines (stack traces, embedded JSON) aren't silently truncated.
// A mid-file read error is logged and the lines gathered so far are returned.
func readLogFileLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		logging.LogError(logging.KeyApp, "failed to read log file %s: %v", filepath.Base(path), err)
	}
	return lines, nil
}

// @Summary Get log file contents
// @Description Parses per-key log file(s) into structured entries, merges + time-sorts them and renders a table (default), a summary (view=summary) or monospace lines (raw=true). name picks one key's file (e.g. file-sync.log), name=all merges every key's log, omitted uses app.log. from/to (unix seconds) filter server-side - the range reaches entries older than a plain load returns.
// @Tags system
// @Produce json,html
// @Param name query string false "log file name, or 'all' to merge every key's log"
// @Param from query int false "only entries at/after this unix-seconds time"
// @Param to query int false "only entries at/before this unix-seconds time"
// @Param raw query bool false "reconstructed monospace line view instead of the table"
// @Param view query string false "'summary' for a compact time+level+message view with a lower default limit"
// @Param limit query int false "max entries to return (table default 5000, or 50000 with from/to, or 20 for view=summary)"
// @Success 200 {string} string "log HTML"
// @Failure 404 {string} string "file logging not enabled"
// @Router /api/logs/file [get]
func handleAPIGetLogsFile(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	summary := q.Get("view") == "summary"
	merged := q.Get("name") == "all"

	from, hasFrom := parseUnixTimeParam(q.Get("from"))
	to, hasTo := parseUnixTimeParam(q.Get("to"))
	ranged := hasFrom || hasTo

	// perFile caps how many recent entries each file contributes to a *merge*,
	// so one chatty file (app.log) can't crowd quieter per-key files out of the
	// newest-N result. It doesn't apply to a single file or a from/to range
	// (there the range bounds the result). limit is the final cap.
	perFile, limit := 300, 5000
	switch {
	case summary:
		perFile, limit = 20, 20 // compact rail sidebar view - not the full history
	case ranged:
		limit = 50000
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	var paths []string
	if merged {
		dir := logging.GetLogsDir()
		for _, name := range logging.GetAllLogFiles() {
			if strings.HasSuffix(name, ".log") { // skip rotated .log.N parts
				paths = append(paths, filepath.Join(dir, name))
			}
		}
	} else {
		path := resolveLogFilePath(r)
		if path == "" {
			writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "file logging not enabled"))
			return
		}
		paths = []string{path}
	}

	var entries []logging.LogEntry
	for _, path := range paths {
		lines, err := readLogFileLines(path)
		if err != nil {
			if !merged { // a single file that won't open is an error, not an empty view
				logging.LogError(logging.KeyApp, "failed to open log file: %v", err)
				writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to open log file"))
				return
			}
			continue
		}

		key := logging.KeyApp
		if base := strings.TrimSuffix(filepath.Base(path), ".log"); base != "app" {
			key = logging.Key(base)
		}
		fileEntries := render.ParseLogLines(key, lines)
		if merged {
			// session separators only make sense per file - across the merge
			// they interleave into noise
			fileEntries = slices.DeleteFunc(fileEntries, func(e logging.LogEntry) bool { return e.Level == "session" })
		}
		if ranged {
			// entries are chronological within a file, so this keeps only the
			// windowed slice - the merged set never grows past the range
			fileEntries = slices.DeleteFunc(fileEntries, func(e logging.LogEntry) bool {
				return (hasFrom && e.Time.Before(from)) || (hasTo && e.Time.After(to))
			})
		} else if merged && len(fileEntries) > perFile {
			fileEntries = fileEntries[len(fileEntries)-perFile:]
		}
		entries = append(entries, fileEntries...)
	}

	// no paging: entries past this cap are only reachable by narrowing from/to
	sort.Slice(entries, func(i, j int) bool { return entries[i].Time.Before(entries[j].Time) })
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	switch {
	case summary:
		writeResponse(w, r, entries, render.RenderLogTableSummary(entries))
	case q.Get("raw") == "true":
		writeResponse(w, r, entries, render.RawLogLines(entries))
	default:
		writeResponse(w, r, entries, render.RenderLogTable(entries))
	}
}

func resolveLogFilePath(r *http.Request) string {
	name := r.URL.Query().Get("name")
	if name == "" {
		return logging.GetLogFilePath()
	}
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

// @Summary Get job history
// @Description Returns recent job runs as HTML table (for HTMX) or JSON
// @Tags system
// @Produce json,html
// @Param sort query string false "sort column: job, started, finished, duration, status"
// @Param dir query string false "sort direction: asc (default) or desc"
// @Success 200 {array} job.JobRun
// @Router /api/system/jobs [get]
func handleAPIGetJobs(w http.ResponseWriter, r *http.Request) {
	sortKey := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	runs := job.GetHistory(100)
	job.SortRuns(runs, sortKey, sortDir)
	writeResponse(w, r, runs, render.RenderJobsTable(runs, sortKey, sortDir))
}

// @Summary Get version/build info
// @Description Returns the version/build-info table as HTML (for HTMX) or JSON - the same content shown on the /system/version page, for embedding in the rail "version" panel
// @Tags system
// @Produce json,html
// @Success 200 {object} render.VersionInfo
// @Router /api/system/version [get]
func handleAPIGetSystemVersion(w http.ResponseWriter, r *http.Request) {
	writeResponse(w, r, render.GetVersionInfo(), render.RenderVersionInfo(true))
}

// @Summary Get changelog
// @Description Returns the rendered changelog history as HTML (for HTMX) - the same content shown on the /system/changelog page, for embedding in the rail "changelog" panel
// @Tags system
// @Produce html
// @Success 200 {string} string "changelog HTML"
// @Router /api/system/changelog [get]
func handleAPIGetSystemChangelog(w http.ResponseWriter, r *http.Request) {
	html, _ := render.RenderChangelog()
	writeResponse(w, r, nil, html)
}

// @Summary Get release notes
// @Description Returns the rendered version info and release notes as HTML (for HTMX) - the same content shown on the /system/release page, for embedding in the rail "release" panel
// @Tags system
// @Produce html
// @Success 200 {string} string "release HTML"
// @Router /api/system/release [get]
func handleAPIGetSystemRelease(w http.ResponseWriter, r *http.Request) {
	html, _ := render.RenderRelease()
	writeResponse(w, r, nil, html)
}

// @Summary Get environment variables
// @Description Returns every recognized KNOV_* environment variable with description, default and current value (sensitive values redacted) as JSON, a full HTML table (for HTMX, the same content shown on the /system/environment page), or - with view=summary - a compact "KEY: value" list (for the admin panel's environment section)
// @Tags system
// @Produce json,html
// @Param view query string false "html view: full table (default) or 'summary' for a compact key/value list"
// @Success 200 {array} render.EnvVarInfo
// @Router /api/system/environment [get]
func handleAPIGetSystemEnvironment(w http.ResponseWriter, r *http.Request) {
	html := render.RenderEnvironmentTable()
	if r.URL.Query().Get("view") == "summary" {
		html = render.RenderEnvironmentSummary()
	}
	writeResponse(w, r, render.GetEnvironmentInfo(), html)
}

// @Summary Download a log file
// @Description Downloads the raw contents of a single log file as plain text
// @Tags system
// @Produce plain
// @Param name query string false "log file name (default: the active app.log)"
// @Success 200 {file} file "log file contents"
// @Failure 404 {string} string "file logging not enabled"
// @Router /api/logs/download [get]
func handleAPIDownloadLogs(w http.ResponseWriter, r *http.Request) {
	path := resolveLogFilePath(r)
	if path == "" {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "file logging not enabled"))
		return
	}

	f, err := os.Open(path)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to open log file for download: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to open log file"))
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(path)))
	io.Copy(w, f)
}
