package server

import (
	"net/http"
	"os"
	"strconv"

	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/renderer"
	"knov/internal/utils"
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
		http.Error(w, "filepath parameter required", http.StatusBadRequest)
		return
	}

	// parse pagination params
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

	// parse sort params
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

	// parse search param
	searchQuery := r.URL.Query().Get("search")

	// read file content
	fullPath := utils.ToFullPath(filepath)
	fileContent, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", fullPath, err)
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	// detect file type and parse table
	fileType := parser.DetectFileType(filepath, string(fileContent))

	var tableData *parser.TableData
	switch fileType {
	case "dokuwiki":
		tableData, err = parser.ParseDokuWikiTable(string(fileContent))
		if err != nil {
			logging.LogError("failed to parse dokuwiki table: %v", err)
			http.Error(w, "failed to parse table", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "table parsing not supported for this file type", http.StatusBadRequest)
		return
	}

	// apply search
	if searchQuery != "" {
		tableData = parser.SearchTable(tableData, searchQuery)
	}

	// apply sorting
	if sortCol >= 0 {
		tableData = parser.SortTable(tableData, sortCol, sortOrder)
	}

	// apply pagination
	paginatedData := parser.PaginateTable(tableData, page, size)

	// render html
	html := renderer.RenderTableHTML(paginatedData, filepath, page, size, sortCol, sortOrder, searchQuery)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
