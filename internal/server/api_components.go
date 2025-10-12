package server

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"knov/internal/files"
	"knov/internal/filetype"
	"knov/internal/logging"
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

	fullPath := utils.ToFullPath(filepath)
	fileContent, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", fullPath, err)
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	handler := files.GetFileTypeRegistry().GetHandler(fullPath)
	if handler == nil {
		http.Error(w, "unsupported file type", http.StatusBadRequest)
		return
	}

	var tableData *filetype.TableData
	if dokuwikiHandler, ok := handler.(*filetype.DokuwikiHandler); ok {
		tableData, err = dokuwikiHandler.ParseDokuWikiTable(string(fileContent))
		if err != nil {
			logging.LogError("failed to parse dokuwiki table: %v", err)
			http.Error(w, "failed to parse table", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "table parsing not supported for this file type", http.StatusBadRequest)
		return
	}

	if searchQuery != "" {
		tableData = filetype.SearchTable(tableData, searchQuery)
	}

	if sortCol >= 0 {
		tableData = filetype.SortTable(tableData, sortCol, sortOrder)
	}

	paginatedData := filetype.PaginateTable(tableData, page, size)

	html := filetype.RenderTableHTML(paginatedData, filepath, page, size, sortCol, sortOrder, searchQuery)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get file editor
// @Tags components
// @Param filepath query string true "File path"
// @Produce html
// @Router /api/components/editor [get]
func handleAPIGetEditor(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, "missing filepath parameter", http.StatusBadRequest)
		return
	}

	fullPath := utils.ToFullPath(filepath)
	content, err := files.GetRawContent(fullPath)
	if err != nil {
		content = "" // empty for new files
	}

	html := fmt.Sprintf(`
		<div id="component-editor">
			<form hx-post="/api/files/save" hx-target="#editor-status">
				<input type="hidden" name="filepath" value="%s">
				<textarea name="content" rows="25" style="width: 100%%; font-family: monospace; padding: 12px;">%s</textarea>
				<div style="margin-top: 12px;">
					<button type="submit" class="btn-primary">save</button>
					<button type="button" onclick="location.reload()" class="btn-secondary">cancel</button>
				</div>
			</form>
			<div id="editor-status"></div>
		</div>
	`, filepath, content)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
