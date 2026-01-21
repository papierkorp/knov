package server

import (
	"net/http"
	"os"
	"strconv"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/server/render"
	"knov/internal/translation"
	"knov/internal/types"
)

// @Summary Get paginated table
// @Description Returns paginated, sortable, searchable table HTML fragment
// @Tags components
// @Param filepath query string true "File path"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Items per page" default(25)
// @Param sort query int false "Column index to sort by" default(-1)
// @Param order query string false "Sort order (asc/desc)" default(asc)
// @Param search query string false "Search query"
// @Produce text/html
// @Success 200 {string} string "table html fragment"
// @Failure 400 {string} string "invalid parameters"
// @Failure 500 {string} string "failed to process table"
// @Router /api/components/table [get]
func handleAPIGetTable(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "filepath parameter required"), http.StatusBadRequest)
		return
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	size := 25
	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			size = s
		}
	}

	sortCol := -1
	if sortStr := r.URL.Query().Get("sort"); sortStr != "" {
		if s, err := strconv.Atoi(sortStr); err == nil && s >= 0 {
			sortCol = s
		}
	}

	sortOrder := "asc"
	if order := r.URL.Query().Get("order"); order == "desc" {
		sortOrder = "desc"
	}

	searchQuery := r.URL.Query().Get("search")

	fullPath := contentStorage.ToDocsPath(filepath)
	fileContent, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusInternalServerError)
		return
	}

	handler := parser.GetParserRegistry().GetHandler(fullPath)
	if handler == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "unsupported file type"), http.StatusBadRequest)
		return
	}

	var tableData *types.TableData
	if dokuwikiHandler, ok := handler.(*parser.DokuwikiHandler); ok {
		tableData, err = dokuwikiHandler.ParseDokuWikiTable(string(fileContent))
		if err != nil {
			logging.LogError("failed to parse dokuwiki table: %v", err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse table"), http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "table parsing not supported for this file type"), http.StatusBadRequest)
		return
	}

	if searchQuery != "" {
		tableData = parser.SearchTable(tableData, searchQuery)
	}

	if sortCol >= 0 {
		tableData = parser.SortTable(tableData, sortCol, sortOrder)
	}

	paginatedData := parser.PaginateTable(tableData, page, size)

	html := render.RenderTableComponent(paginatedData, filepath, page, size, sortCol, sortOrder, searchQuery)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
