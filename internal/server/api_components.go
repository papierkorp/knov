package server

import (
	"net/http"
	"os"
	"strconv"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/server/render"
	"knov/internal/translation"
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

	fullPath := utils.ToFullPath(filepath)
	fileContent, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusInternalServerError)
		return
	}

	handler := files.GetParserRegistry().GetHandler(fullPath)
	if handler == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "unsupported file type"), http.StatusBadRequest)
		return
	}

	var tableData *parser.TableData
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

// @Summary Get table editor
// @Description Returns table editor HTML with jspreadsheet
// @Tags components
// @Param filepath query string true "File path"
// @Produce text/html
// @Success 200 {string} string "table editor html"
// @Failure 400 {string} string "invalid parameters"
// @Failure 500 {string} string "failed to read file"
// @Router /api/components/table/editor [get]
func handleAPIGetTableEditor(w http.ResponseWriter, r *http.Request) {
	filepath := r.URL.Query().Get("filepath")
	if filepath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "filepath parameter required"), http.StatusBadRequest)
		return
	}

	fullPath := utils.ToFullPath(filepath)
	fileContent, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusInternalServerError)
		return
	}

	handler := files.GetParserRegistry().GetHandler(fullPath)
	if handler == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "unsupported file type"), http.StatusBadRequest)
		return
	}

	var tableData *parser.TableData
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

	html := render.RenderTableEditor(tableData, filepath)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Update table
// @Description Updates table content from jspreadsheet editor
// @Tags components
// @Accept application/x-www-form-urlencoded
// @Param filepath formData string true "File path"
// @Param data formData string true "Table data as JSON"
// @Produce text/html
// @Success 200 {string} string "updated table html"
// @Failure 400 {string} string "invalid parameters"
// @Failure 500 {string} string "failed to save table"
// @Router /api/components/table [put]
func handleAPIPutTable(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	filepath := r.FormValue("filepath")
	dataJSON := r.FormValue("data")

	if filepath == "" || dataJSON == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "filepath and data required"), http.StatusBadRequest)
		return
	}

	fullPath := utils.ToFullPath(filepath)
	handler := files.GetParserRegistry().GetHandler(fullPath)
	if handler == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "unsupported file type"), http.StatusBadRequest)
		return
	}

	dokuwikiHandler, ok := handler.(*parser.DokuwikiHandler)
	if !ok {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "table conversion not supported for this file type"), http.StatusBadRequest)
		return
	}

	tableSyntax, err := dokuwikiHandler.ConvertJSONToDokuWikiTable(dataJSON)
	if err != nil {
		logging.LogError("failed to convert json to dokuwiki table: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to convert table"), http.StatusInternalServerError)
		return
	}

	fileContent, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogError("failed to read file %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusInternalServerError)
		return
	}

	updatedContent := dokuwikiHandler.ReplaceTableInContent(string(fileContent), tableSyntax)

	if err := os.WriteFile(fullPath, []byte(updatedContent), 0644); err != nil {
		logging.LogError("failed to write file %s: %v", fullPath, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("table updated successfully: %s", filepath)

	// return updated table view
	tableData, _ := dokuwikiHandler.ParseDokuWikiTable(tableSyntax)
	html := render.RenderTableComponent(tableData, filepath, 1, 25, -1, "asc", "")

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
