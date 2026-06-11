// Package server - kanban board API handlers
package server

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/filter"
	"knov/internal/kanban"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"

	"github.com/go-chi/chi/v5"
)

// @Summary Get kanban board for a collection
// @Description Returns all kanban cards grouped by status column for the given collection
// @Tags kanban
// @Param collection path string true "Collection name"
// @Param ancestor query string false "Filter by ancestor (epic)"
// @Param tag query string false "Filter by tag"
// @Param q query string false "Search query"
// @Produce json,html
// @Router /api/kanban/{collection} [get]
func handleAPIGetKanbanBoard(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	if collection == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing collection"), http.StatusBadRequest)
		return
	}

	cfg := &filter.Config{Logic: "and"}
	cfg.Criteria = append(cfg.Criteria, filter.Criteria{Metadata: "collection", Operator: "equals", Value: collection, Action: "include"})

	if ancestor := r.URL.Query().Get("ancestor"); ancestor != "" {
		cfg.Criteria = append(cfg.Criteria, filter.Criteria{Metadata: "ancestor-of", Operator: "equals", Value: ancestor, Action: "include"})
	}
	if tag := r.URL.Query().Get("tag"); tag != "" {
		cfg.Criteria = append(cfg.Criteria, filter.Criteria{Metadata: "tags", Operator: "equals", Value: tag, Action: "include"})
	}

	cols, _ := kanban.BuildBoard(collection, cfg, strings.ToLower(r.URL.Query().Get("q")), kanban.SortBy(r.URL.Query().Get("sort")))
	writeResponse(w, r, cols, render.RenderKanbanBoard(cols))
}

// @Summary Apply advanced filter to kanban board
// @Description Filters the kanban board using the full filter form; collection is always injected as the first criterion
// @Tags kanban
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param collection path string true "Collection name"
// @Success 200 {string} string "kanban board html"
// @Router /api/kanban/{collection}/filter [post]
func handleAPIPostKanbanFilter(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	if collection == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing collection"), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	cfg := filter.ParseFilterConfigFromForm(r, -1)
	cfg.Criteria = append([]filter.Criteria{{Metadata: "collection", Operator: "equals", Value: collection, Action: "include"}}, cfg.Criteria...)
	cfg.Logic = "and"

	cols, _ := kanban.BuildBoard(collection, cfg, "", kanban.SortBy(r.FormValue("sort")))
	writeResponse(w, r, cols, render.RenderKanbanBoard(cols))
}

// @Summary Move a kanban card to a new status column
// @Description Updates the kanban status tag on a file, replacing any existing kanban tag
// @Tags kanban
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param filepath formData string true "File path"
// @Param status formData string true "New kanban status"
// @Success 200 {string} string "card updated"
// @Failure 400 {string} string "missing parameter"
// @Failure 404 {string} string "file not found"
// @Failure 500 {string} string "failed to update"
// @Router /api/kanban/card/move [post]
func handleAPIKanbanMoveCard(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("filepath")
	newStatus := r.FormValue("status")

	if filePath == "" || newStatus == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath or status"), http.StatusBadRequest)
		return
	}
	if !slices.Contains(configmanager.GetKanbanStatuses(), newStatus) {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid kanban status"), http.StatusBadRequest)
		return
	}

	oldStatus, err := kanban.MoveCard(filePath, newStatus)
	if err != nil {
		logging.LogError("failed to move kanban card %s to %s: %v", filePath, newStatus, err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to update card"))
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to update card"), http.StatusInternalServerError)
		return
	}

	var msg string
	if oldStatus == "" {
		msg = translation.SprintfForRequest(configmanager.GetLanguage(), "status added: %s", newStatus)
	} else {
		msg = translation.SprintfForRequest(configmanager.GetLanguage(), "status changed: %s → %s", oldStatus, newStatus)
	}
	notify.SetHeader(w, notify.LevelSuccess, msg)
	writeResponse(w, r, map[string]string{"filepath": filePath, "status": newStatus}, "")
}

// @Summary Save card order for a kanban column
// @Description Persists the drag-and-drop card order for all columns in a collection
// @Tags kanban
// @Accept application/x-www-form-urlencoded
// @Param collection path string true "Collection name"
// @Param status formData string true "Column status"
// @Param order formData string true "Comma-separated list of filepaths in display order"
// @Success 200 {string} string "order saved"
// @Router /api/kanban/{collection}/order [post]
func handleAPIKanbanSaveOrder(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	if collection == "" {
		http.Error(w, "missing collection", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	status := r.FormValue("status")
	if status == "" {
		http.Error(w, "missing status", http.StatusBadRequest)
		return
	}

	var paths []string
	for _, p := range strings.Split(r.FormValue("order"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			paths = append(paths, p)
		}
	}

	stored, err := kanban.GetOrder(collection)
	if err != nil {
		logging.LogError("kanban: load order failed for %s: %v", collection, err)
		stored = kanban.Order{}
	}
	stored[status] = paths

	if err := kanban.SaveOrder(collection, stored); err != nil {
		logging.LogError("kanban: save order failed for %s: %v", collection, err)
		http.Error(w, "failed to save order", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// @Summary Get a short text excerpt from a file
// @Description Returns the first N runes of meaningful body text, stripping front matter and markdown syntax
// @Tags kanban
// @Param filepath query string true "File path (relative)"
// @Param chars query int false "Max runes to return (default 30)"
// @Produce html
// @Success 200 {string} string "excerpt text"
// @Router /api/kanban/excerpt [get]
func handleAPIGetKanbanExcerpt(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(""))
		return
	}

	chars := 30
	if c := r.URL.Query().Get("chars"); c != "" {
		if n, err := strconv.Atoi(c); err == nil && n > 0 {
			chars = n
		}
	}

	excerpt := kanban.Excerpt(pathutils.ToDocsPath(filePath), chars)

	w.Header().Set("Content-Type", "text/html")
	if excerpt != "" {
		fmt.Fprintf(w, `<div class="kanban-card-excerpt">%s</div>`, excerpt)
	}
}

// @Summary Get all non-kanban tags used in a collection's kanban cards
// @Tags kanban
// @Param collection path string true "Collection name"
// @Produce html
// @Router /api/kanban/{collection}/tags [get]
func handleAPIGetKanbanTags(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	if collection == "" {
		w.Write([]byte(""))
		return
	}

	tags, err := kanban.TagsForCollection(collection)
	if err != nil {
		w.Write([]byte(""))
		return
	}

	var html strings.Builder
	for _, t := range tags {
		fmt.Fprintf(&html, `<option value="%s">%s</option>`, t, t)
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html.String()))
}
