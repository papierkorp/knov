// Package server ..
package server

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to invalidate cache"))
		http.Error(w, "failed to invalidate cache", http.StatusInternalServerError)
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "cache invalidated"))
	writeResponse(w, r, map[string]string{"status": "cache invalidated"}, "")
}

// @Summary Get recent log entries
// @Description Returns the most recent in-memory log entries across every key as an HTML table, newest first. Powers the "Live" view on the admin logs page.
// @Tags system
// @Produce html
// @Success 200 {string} string "log table HTML"
// @Router /api/logs [get]
func handleAPIGetLogs(w http.ResponseWriter, r *http.Request) {
	entries := logging.GetRecentEntries(200)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(render.RenderLogTable(entries)))
}

// handleAPIGetLogsFileAll merges every current (non-rotated) per-key log file
// into one chronologically sorted table - the "All (merged)" file-view option.
// Unlike a single file, this reads and re-sorts on every request rather than
// supporting the "load earlier lines" chunking handleAPIGetLogsFile does.
func handleAPIGetLogsFileAll(w http.ResponseWriter, r *http.Request) {
	limit := 1000
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	dir := logging.GetLogsDir()
	var entries []logging.LogEntry
	for _, name := range logging.GetAllLogFiles() {
		if !strings.HasSuffix(name, ".log") {
			continue // skip rotated .log.N parts
		}

		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var lines []string
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		f.Close()

		key := logging.KeyApp
		if base := strings.TrimSuffix(name, ".log"); base != "app" {
			key = logging.Key(base)
		}
		entries = append(entries, render.ParseLogLines(key, lines)...)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Time.Before(entries[j].Time) })
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(render.RenderLogTable(entries)))
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

// @Summary Get log file contents
// @Description Returns lines from a log file for the file-view tab. name selects a single key's log file (e.g. file-sync.log), name=all merges every current key's log into one chronologically sorted table, or name is omitted for the active app.log. chunk/limit/offset page through a single file (not supported for name=all).
// @Tags system
// @Produce html
// @Param name query string false "log file name, or 'all' to merge every key's log"
// @Param limit query int false "max lines/entries to return (default 1000)"
// @Param offset query int false "lines to skip from the end, for paging a single file"
// @Param chunk query bool false "return only the appended fragment, without the surrounding container"
// @Success 200 {string} string "log lines HTML"
// @Failure 404 {string} string "file logging not enabled"
// @Router /api/logs/file [get]
func handleAPIGetLogsFile(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("name") == "all" {
		handleAPIGetLogsFileAll(w, r)
		return
	}

	path := resolveLogFilePath(r)
	if path == "" {
		http.Error(w, "file logging not enabled", http.StatusNotFound)
		return
	}

	limit := 1000
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	isChunk := r.URL.Query().Get("chunk") == "true"

	f, err := os.Open(path)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to open log file: %v", err)
		http.Error(w, "failed to open log file", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	total := len(lines)
	end := total - offset
	if end < 0 {
		end = 0
	}
	start := end - limit
	if start < 0 {
		start = 0
	}
	chunk := lines[start:end]
	hasMore := start > 0

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Log-Total-Lines", strconv.Itoa(total))
	if hasMore {
		w.Header().Set("X-Log-Has-More", "true")
	}

	var sb strings.Builder
	if isChunk {
		sb.WriteString(render.LogFileLines(chunk))
	} else {
		sb.WriteString(`<div id="log-file-container">`)
		if hasMore {
			sb.WriteString(fmt.Sprintf(
				`<div id="log-more-area"><button class="btn-secondary" onclick="loadMoreLogLines()">Load earlier lines</button> <span id="log-line-info" class="log-line-info">showing last %d of %d lines</span></div>`,
				end-start, total,
			))
		} else {
			sb.WriteString(`<div id="log-more-area" style="display:none"><button class="btn-secondary" onclick="loadMoreLogLines()">Load earlier lines</button></div>`)
		}
		sb.WriteString(`<div class="log-file-lines" id="log-file-lines">`)
		sb.WriteString(render.LogFileLines(chunk))
		sb.WriteString(`</div></div>`)
	}

	w.Write([]byte(sb.String()))
}

// @Summary Get job history
// @Description Returns recent job runs as HTML table (for HTMX) or JSON
// @Tags system
// @Produce json,html
// @Success 200 {array} job.JobRun
// @Router /api/system/jobs [get]
func handleAPIGetJobs(w http.ResponseWriter, r *http.Request) {
	runs := job.GetRecentRuns()
	acceptHeader := r.Header.Get("Accept")
	if strings.Contains(acceptHeader, "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(render.RenderJobsTable(runs)))
		return
	}
	writeResponse(w, r, runs, render.RenderJobsTable(runs))
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
		http.Error(w, "file logging not enabled", http.StatusNotFound)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to open log file for download: %v", err)
		http.Error(w, "failed to open log file", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(path)))
	io.Copy(w, f)
}
