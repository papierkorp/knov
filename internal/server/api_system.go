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
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/server/notify"
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
	if err := files.CacheInvalidate(); err != nil {
		logging.LogError("failed to invalidate cache: %v", err)
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
	sb.WriteString(`<table class="log-table"><thead><tr><th>Time</th><th>Level</th><th>Caller</th><th>Message</th></tr></thead><tbody>`)
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		sb.WriteString(fmt.Sprintf(
			`<tr class="log-level-%s"><td>%s</td><td>%s</td><td class="log-caller">%s</td><td>%s</td></tr>`,
			html.EscapeString(e.Level),
			html.EscapeString(configmanager.FormatDateTimeSeconds(e.Time)),
			html.EscapeString(e.Level),
			html.EscapeString(e.Caller),
			html.EscapeString(e.Message),
		))
	}
	sb.WriteString(`</tbody></table>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(sb.String()))
}

func handleAPIGetLogsFile(w http.ResponseWriter, r *http.Request) {
	path := logging.GetLogFilePath()
	if path == "" {
		http.Error(w, "file logging not enabled", http.StatusNotFound)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		logging.LogError("failed to open log file: %v", err)
		http.Error(w, "failed to open log file", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	var sb strings.Builder
	sb.WriteString(`<div class="log-file-lines">`)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		sb.WriteString(`<div class="log-line">`)
		sb.WriteString(html.EscapeString(line))
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</div>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(sb.String()))
}

func handleAPIDownloadLogs(w http.ResponseWriter, r *http.Request) {
	path := logging.GetLogFilePath()
	if path == "" {
		http.Error(w, "file logging not enabled", http.StatusNotFound)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		logging.LogError("failed to open log file for download: %v", err)
		http.Error(w, "failed to open log file", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(path)))
	io.Copy(w, f)
}
