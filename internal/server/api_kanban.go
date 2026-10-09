// Package server - kanban board API handlers
package server

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/filter"
	"knov/internal/job"
	"knov/internal/kanban"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"

	"github.com/go-chi/chi/v5"
)

// resolveBoard looks up a configured kanban board by its URL slug, writing a 404 if unknown.
func resolveBoard(w http.ResponseWriter, r *http.Request) (configmanager.KanbanBoard, bool) {
	slug := chi.URLParam(r, "board")
	if slug == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing board"))
		return configmanager.KanbanBoard{}, false
	}
	board, ok := configmanager.GetKanbanBoardBySlug(slug)
	if !ok {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "unknown board"))
		return configmanager.KanbanBoard{}, false
	}
	return board, true
}

// @Summary Trigger a manual file-sync
// @Description Starts the file-sync job (the same one the scheduler runs periodically) in the
// @Description background, so files moved or edited outside the app are picked up before
// @Description dragging their cards; poll the returned job id via GET /api/jobs/{id} for
// @Description completion. Its dedup lock already rejects a second concurrent run outright, so
// @Description repeated presses just report "already running" instead of queuing up.
// @Tags kanban
// @Produce json,html
// @Success 200 {object} jobStorage.JobRecord
// @Failure 409 {string} string "sync already running"
// @Router /api/kanban/sync [post]
func handleAPIKanbanSync(w http.ResponseWriter, r *http.Request) {
	id, err := job.StartFileSyncManual()
	if err != nil {
		msg := translation.SprintfForRequest(configmanager.GetLanguage(), "sync already running")
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrAlreadyRunning) {
			status = http.StatusConflict
		}
		writeAPIError(w, r, status, msg)
		return
	}

	respondJobStarted(w, r, id, job.JobTypeFileSync)
}

// @Summary Scan for kanban tag issues
// @Description Lists files with several status tags, statuses not in the status list, unknown kanban-prefixed tags, cards whose foldersync status folder disagrees with their tag and cards outside any board. Issues with a fix can be cleaned up via POST /api/metadata/kanban-issues/cleanup.
// @Tags kanban
// @Produce json,html
// @Success 200 {array} kanban.Issue
// @Failure 500 {string} string "failed to scan for kanban issues"
// @Router /api/metadata/kanban-issues [get]
func handleAPIGetKanbanIssues(w http.ResponseWriter, r *http.Request) {
	issues, err := kanban.ScanIssues()
	if err != nil {
		logging.LogError(logging.KeyKanbanCleanup, "failed to scan for kanban issues: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to scan for kanban issues"))
		return
	}
	writeResponse(w, r, issues, render.RenderKanbanIssues(issues))
}

// @Summary Clean up kanban tag issues
// @Description Sets the status of every fixable kanban issue (foldersync: the status folder, several status tags: the first valid one, the column the board shows) and returns the rescanned list
// @Tags kanban
// @Produce json,html
// @Success 200 {object} kanban.CleanupResult
// @Failure 500 {string} string "failed to clean up kanban issues"
// @Router /api/metadata/kanban-issues/cleanup [post]
func handleAPICleanupKanbanIssues(w http.ResponseWriter, r *http.Request) {
	lang := configmanager.GetLanguage()
	result, err := kanban.CleanupIssues()
	if err != nil {
		logging.LogError(logging.KeyKanbanCleanup, "failed to clean up kanban issues: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(lang, "failed to clean up kanban issues"))
		return
	}

	level := notify.LevelSuccess
	if result.Failed > 0 {
		level = notify.LevelError
	}
	notify.SetHeader(w, level, translation.SprintfForRequest(lang, "%d files fixed, %d failed", result.Fixed, result.Failed))

	// the cleanup itself succeeded, so a failed rescan only replaces the list
	html := render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(lang, "failed to scan for kanban issues"))
	if issues, err := kanban.ScanIssues(); err != nil {
		logging.LogError(logging.KeyKanbanCleanup, "failed to rescan for kanban issues: %v", err)
	} else {
		html = render.RenderKanbanIssues(issues)
	}
	writeResponse(w, r, result, html)
}

// @Summary Get kanban board for a folder
// @Description Returns all kanban cards grouped by status column for the given board
// @Tags kanban
// @Param board path string true "Board slug"
// @Param ancestor query string false "Filter by ancestor (epic)"
// @Param tag query string false "Filter by tag"
// @Param q query string false "Search query"
// @Produce json,html
// @Router /api/kanban/{board} [get]
func handleAPIGetKanbanBoard(w http.ResponseWriter, r *http.Request) {
	board, ok := resolveBoard(w, r)
	if !ok {
		return
	}

	cfg := &filter.Config{Logic: "and"}

	if ancestor := r.URL.Query().Get("ancestor"); ancestor != "" {
		cfg.Criteria = append(cfg.Criteria, filter.Criteria{Metadata: "ancestor-of", Operator: "equals", Value: ancestor, Action: "include"})
	}
	if tag := r.URL.Query().Get("tag"); tag != "" {
		cfg.Criteria = append(cfg.Criteria, filter.Criteria{Metadata: "tags", Operator: "equals", Value: tag, Action: "include"})
	}

	cols, _ := kanban.BuildBoard(board.FolderPath, cfg, strings.ToLower(r.URL.Query().Get("q")), kanban.SortBy(r.URL.Query().Get("sort")))
	writeResponse(w, r, cols, render.RenderKanbanBoard(cols, board))
}

// @Summary Get archived kanban cards for a board
// @Description Returns all cards with the archive status for the board's folder (and subfolders), newest first
// @Tags kanban
// @Param board path string true "Board slug"
// @Produce json,html
// @Router /api/kanban/{board}/archive [get]
func handleAPIGetKanbanArchive(w http.ResponseWriter, r *http.Request) {
	board, ok := resolveBoard(w, r)
	if !ok {
		return
	}

	cards, err := kanban.Archived(board.FolderPath)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get archived cards for %s: %v", board.FolderPath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get archived cards"))
		return
	}

	writeResponse(w, r, cards, render.RenderKanbanArchive(cards, board))
}

// @Summary Apply advanced filter to kanban board
// @Description Filters the kanban board using the full filter form, scoped to the board's folder
// @Tags kanban
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param board path string true "Board slug"
// @Success 200 {string} string "kanban board html"
// @Router /api/kanban/{board}/filter [post]
func handleAPIPostKanbanFilter(w http.ResponseWriter, r *http.Request) {
	board, ok := resolveBoard(w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	cfg := filter.ParseFilterConfigFromForm(r, -1)
	cfg.Logic = "and"

	cols, _ := kanban.BuildBoard(board.FolderPath, cfg, "", kanban.SortBy(r.FormValue("sort")))
	writeResponse(w, r, cols, render.RenderKanbanBoard(cols, board))
}

// @Summary Move a kanban card to a new status column
// @Description Updates the kanban status tag on a file, replacing any existing kanban tag
// @Tags kanban
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path (docs/ prefixed)"
// @Param status formData string true "New kanban status"
// @Param board formData string false "Board slug (scopes the event log entry; omit to guess from the file's folder)"
// @Success 200 {string} string "card updated"
// @Failure 400 {string} string "missing parameter"
// @Failure 404 {string} string "file not found"
// @Failure 500 {string} string "failed to update"
// @Router /api/kanban/card/move [post]
func handleAPIKanbanMoveCard(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	filePath, ok := metaPathParam(w, r, "filepath")
	if !ok {
		return
	}
	newStatus := r.FormValue("status")

	if filePath == "" || newStatus == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath or status"))
		return
	}
	if !slices.Contains(configmanager.GetKanbanStatuses(), newStatus) {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid kanban status"))
		return
	}

	var boardFolder string
	if board, ok := configmanager.GetKanbanBoardBySlug(r.FormValue("board")); ok {
		boardFolder = board.FolderPath
	}

	oldStatus, newFilePath, err := kanban.MoveCard(boardFolder, pathutils.ToRelative(filePath.String()), newStatus)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to move kanban card %s to %s: %v", filePath, newStatus, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to update card"))
		return
	}

	var msg string
	if oldStatus == "" {
		msg = translation.SprintfForRequest(configmanager.GetLanguage(), "status added: %s", newStatus)
	} else {
		msg = translation.SprintfForRequest(configmanager.GetLanguage(), "status changed: %s → %s", oldStatus, newStatus)
	}
	notify.SetHeader(w, notify.LevelSuccess, msg)
	writeResponse(w, r, map[string]string{"filepath": pathutils.DocsPath(newFilePath).String(), "status": newStatus}, "")
}

// @Summary Save card order for a kanban column
// @Description Persists the drag-and-drop card order for all columns in a board
// @Tags kanban
// @Accept application/x-www-form-urlencoded
// @Param board path string true "Board slug"
// @Param status formData string true "Column status"
// @Param order formData string true "Comma-separated list of docs/ prefixed filepaths in display order"
// @Success 200 {string} string "order saved"
// @Router /api/kanban/{board}/order [post]
func handleAPIKanbanSaveOrder(w http.ResponseWriter, r *http.Request) {
	board, ok := resolveBoard(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	status := r.FormValue("status")
	if status == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing status"))
		return
	}

	var paths []string
	for _, p := range strings.Split(r.FormValue("order"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			if !pathutils.IsMetaPath(p) {
				writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "%s must start with docs/ or media/", "order"))
				return
			}
			paths = append(paths, pathutils.ToRelative(p)) // the order is stored with the docs-relative card paths
		}
	}

	err := kanban.MutateOrder(board.FolderPath, func(o kanban.Order) {
		o[status] = paths
	})
	if err != nil {
		logging.LogError(logging.KeyApp, "kanban: save order failed for %s: %v", board.FolderPath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save order"))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// @Summary Get all non-kanban tags used in a board's kanban cards
// @Tags kanban
// @Param board path string true "Board slug"
// @Produce json,html
// @Router /api/kanban/{board}/tags [get]
func handleAPIGetKanbanTags(w http.ResponseWriter, r *http.Request) {
	board, ok := configmanager.GetKanbanBoardBySlug(chi.URLParam(r, "board"))
	if !ok {
		writeResponse(w, r, []string{}, "")
		return
	}

	tags, err := kanban.TagsForFolder(board.FolderPath)
	if err != nil {
		writeResponse(w, r, []string{}, "")
		return
	}

	var html strings.Builder
	for _, t := range tags {
		fmt.Fprintf(&html, `<option value="%s">%s</option>`, t, t)
	}
	writeResponse(w, r, tags, html.String())
}

// @Summary Get kanban event log
// @Description Returns kanban card move events, newest first. All parameters are optional.
// @Tags kanban
// @Param board path string true "Board slug"
// @Param file query string false "Filter by file path"
// @Param from query string false "Start of time range (RFC3339 or YYYY-MM-DD)"
// @Param to query string false "End of time range (RFC3339 or YYYY-MM-DD)"
// @Param limit query int false "Max number of events (default 200, 0 = unlimited)"
// @Produce json,html
// @Router /api/kanban/{board}/events [get]
func handleAPIGetKanbanEvents(w http.ResponseWriter, r *http.Request) {
	board, ok := resolveBoard(w, r)
	if !ok {
		return
	}

	filePath, ok := metaPathParam(w, r, "file")
	if !ok {
		return
	}
	fileRel := relOfMeta(filePath.String())
	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")

	var from, to *time.Time
	if fromRaw != "" {
		if t, err := parseEventBoundary(fromRaw, false); err == nil {
			from = &t
		}
	}
	if toRaw != "" {
		if t, err := parseEventBoundary(toRaw, true); err == nil {
			to = &t
		}
	}

	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n >= 0 {
			limit = n
		}
	}

	events, err := kanban.GetEvents(board.FolderPath, fileRel, from, to, limit)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get kanban events for %s: %v", board.FolderPath, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get events"))
		return
	}

	filePaths, err := kanban.FilesForFolder(board.FolderPath)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get kanban files for %s: %v", board.FolderPath, err)
	}

	writeResponse(w, r, events, render.RenderKanbanEvents(events, filePaths, board.Slug, fileRel, fromRaw, toRaw))
}

// parseEventBoundary parses a time-range boundary as RFC3339, falling back to a bare
// YYYY-MM-DD date (as produced by a native <input type="date">) expanded to the start
// or end of that day.
func parseEventBoundary(s string, endOfDay bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		d = d.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}
	return d, nil
}

// relOfMeta is the docs-relative card path of a metadata path, "" stays "".
func relOfMeta(p string) string {
	if p == "" {
		return ""
	}
	return pathutils.ToRelative(p)
}
