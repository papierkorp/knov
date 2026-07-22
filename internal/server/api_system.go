// Package server ..
package server

import (
	"bufio"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

func handleAPIGetLogs(w http.ResponseWriter, r *http.Request) {
	entries := logging.GetRecentEntries(200)

	var sb strings.Builder
	sb.WriteString(`<table class="log-table"><thead><tr><th>Time</th><th>Level</th><th>Source</th><th>Caller</th><th>Message</th></tr></thead><tbody>`)
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		sb.WriteString(fmt.Sprintf(
			`<tr class="log-level-%s log-key-%s"><td>%s</td><td>%s</td><td>%s</td><td class="log-caller">%s</td><td>%s</td></tr>`,
			html.EscapeString(e.Level),
			html.EscapeString(e.Key.String()),
			html.EscapeString(configmanager.FormatDateTimeSeconds(e.Time)),
			html.EscapeString(e.Level),
			html.EscapeString(e.Key.String()),
			html.EscapeString(e.Caller),
			html.EscapeString(e.Message),
		))
	}
	sb.WriteString(`</tbody></table>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(sb.String()))
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

func handleAPIGetLogsFile(w http.ResponseWriter, r *http.Request) {
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
