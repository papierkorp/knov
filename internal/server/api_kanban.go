// Package server - kanban board API handlers
package server

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
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

	ancestorFilter := r.URL.Query().Get("ancestor")
	tagFilter := r.URL.Query().Get("tag")
	searchQuery := strings.ToLower(r.URL.Query().Get("q"))

	prefix := configmanager.GetKanbanPrefix()
	columns := configmanager.GetKanbanColumns()

	allFiles, err := files.GetAllFiles()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get files"), http.StatusInternalServerError)
		return
	}

	// bucket cards by status, collect unique non-kanban tags
	cardsByStatus := make(map[string][]render.KanbanCard)
	for _, col := range columns {
		cardsByStatus[col] = []render.KanbanCard{}
	}
	tagSet := make(map[string]struct{})

	for _, file := range allFiles {
		if file.Metadata == nil {
			continue
		}
		meta := file.Metadata

		// must be in the right collection
		if meta.Collection != collection {
			continue
		}

		// must have exactly one kanban tag matching a configured status
		status := kanbanStatusFromTags(meta.Tags, prefix)
		if status == "" {
			continue
		}

		// ancestor (epic) filter
		if ancestorFilter != "" {
			found := false
			for _, a := range meta.Ancestor {
				if pathutils.ToRelative(a) == ancestorFilter {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// tag filter
		if tagFilter != "" {
			found := false
			for _, t := range meta.Tags {
				if t == tagFilter {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// search filter
		if searchQuery != "" {
			title := strings.ToLower(meta.Title)
			path := strings.ToLower(file.Path)
			if !strings.Contains(title, searchQuery) && !strings.Contains(path, searchQuery) {
				continue
			}
		}

		card := render.KanbanCard{
			FilePath:   pathutils.ToRelative(file.Path),
			Title:      meta.Title,
			Collection: meta.Collection,
			Status:     status,
			Tags:       meta.Tags,
			CreatedAt:  meta.CreatedAt.Format("2006-01-02"),
			LastEdited: meta.LastEdited.Format("2006-01-02"),
		}
		cardsByStatus[status] = append(cardsByStatus[status], card)

		// collect non-kanban tags for the filter select
		for _, t := range meta.Tags {
			if !configmanager.IsKanbanTag(t) {
				tagSet[t] = struct{}{}
			}
		}
	}

	// sort cards by createdAt within each column
	for col := range cardsByStatus {
		sort.Slice(cardsByStatus[col], func(i, j int) bool {
			return cardsByStatus[col][i].CreatedAt < cardsByStatus[col][j].CreatedAt
		})
	}

	// render columns HTML
	var html strings.Builder
	fmt.Fprintf(&html, `<div class="kanban-board" id="kanban-board">`)
	for _, col := range columns {
		label := col
		html.WriteString(render.RenderKanbanColumn(col, label, cardsByStatus[col]))
	}
	html.WriteString(`</div>`)

	type colData struct {
		Status string
		Cards  []render.KanbanCard
	}
	jsonCols := make([]colData, 0, len(columns))
	for _, col := range columns {
		jsonCols = append(jsonCols, colData{Status: col, Cards: cardsByStatus[col]})
	}

	writeResponse(w, r, jsonCols, html.String())
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

	// validate status is a known kanban status
	if !slices.Contains(configmanager.GetKanbanStatuses(), newStatus) {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid kanban status"), http.StatusBadRequest)
		return
	}

	normalizedPath := pathutils.ToWithPrefix(filePath)
	meta, err := files.MetaDataGet(normalizedPath)
	if err != nil || meta == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "file not found"), http.StatusNotFound)
		return
	}

	// replace kanban tag: remove all existing kanban tags, add new one
	newTag := configmanager.KanbanStatusTag(newStatus)
	var filtered []string
	for _, t := range meta.Tags {
		if !configmanager.IsKanbanTag(t) {
			filtered = append(filtered, t)
		}
	}
	meta.Tags = append(filtered, newTag)

	if err := files.MetaDataSaveRaw(meta); err != nil {
		logging.LogError("failed to move kanban card %s to %s: %v", filePath, newStatus, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to update card"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("moved kanban card %s to status %s", filePath, newStatus)
	writeResponse(w, r, map[string]string{"filepath": filePath, "status": newStatus}, "ok")
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

	fullPath := pathutils.ToDocsPath(filePath)
	excerpt := extractExcerpt(fullPath, chars)

	w.Header().Set("Content-Type", "text/html")
	if excerpt != "" {
		fmt.Fprintf(w, `<div class="kanban-card-excerpt">%s</div>`, excerpt)
	}
}

// @Summary Get all non-kanban tags used in a collection's kanban cards
// @Tags kanban
// @Param collection path string true "Collection name"
// @Param format query string false "format=options returns HTML option elements"
// @Produce html
// @Router /api/kanban/{collection}/tags [get]
func handleAPIGetKanbanTags(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	if collection == "" {
		w.Write([]byte(""))
		return
	}

	prefix := configmanager.GetKanbanPrefix()
	allFiles, err := files.GetAllFiles()
	if err != nil {
		w.Write([]byte(""))
		return
	}

	tagSet := make(map[string]struct{})
	for _, file := range allFiles {
		if file.Metadata == nil || file.Metadata.Collection != collection {
			continue
		}
		if kanbanStatusFromTags(file.Metadata.Tags, prefix) == "" {
			continue
		}
		for _, t := range file.Metadata.Tags {
			if !configmanager.IsKanbanTag(t) {
				tagSet[t] = struct{}{}
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	slices.Sort(tags)

	var html strings.Builder
	for _, t := range tags {
		fmt.Fprintf(&html, `<option value="%s">%s</option>`, t, t)
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html.String()))
}

// kanbanStatusFromTags extracts the kanban status value from a tag list.
// returns "" if no kanban tag is present.
func kanbanStatusFromTags(tags []string, prefix string) string {
	statusPrefix := prefix + "-status-"
	for _, t := range tags {
		if strings.HasPrefix(t, statusPrefix) {
			return strings.TrimPrefix(t, statusPrefix)
		}
	}
	return ""
}

// extractExcerpt reads a file, strips front matter and markdown syntax, and
// returns the first maxRunes runes of meaningful body text.
func extractExcerpt(fullPath string, maxRunes int) string {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}

	body := parser.StripFrontMatter(data)
	lines := strings.Split(string(body), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// skip markdown headings, horizontal rules, code fences
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "```") {
			continue
		}
		// strip common inline markdown: bold, italic, links, images
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "__", "")
		line = strings.ReplaceAll(line, "*", "")
		line = strings.ReplaceAll(line, "_", "")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if utf8.RuneCountInString(line) <= maxRunes {
			return line
		}
		runes := []rune(line)
		return string(runes[:maxRunes]) + "…"
	}
	return ""
}
