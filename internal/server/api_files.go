package server

import (
	"fmt"
	"net/http"
	"strings"

	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/search"
	"knov/internal/utils"
)

// @Summary Get all files
// @Tags files
// @Produce json,html
// @Router /api/files/list [get]
func handleAPIGetAllFiles(w http.ResponseWriter, r *http.Request) {
	allFiles, err := files.GetAllFiles()
	if err != nil {
		http.Error(w, "failed to get files", http.StatusInternalServerError)
		return
	}

	var html strings.Builder
	html.WriteString("<ul>")
	for _, file := range allFiles {
		html.WriteString(fmt.Sprintf(`<li><a href="#" hx-get="/files/%s?snippet=true" hx-target="#file-content">%s</a></li>`,
			file.Path,
			file.Path))
	}
	html.WriteString("</ul>")

	writeResponse(w, r, allFiles, html.String())
}

// @Summary Get file content as html
// @Tags files
// @Param filepath path string true "File path"
// @Produce text/html
// @Router /api/files/content/{filepath} [get]
func handleAPIGetFileContent(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/content/")
	fullPath := utils.ToFullPath(filePath)

	html, err := files.GetFileContent(fullPath)
	if err != nil {
		http.Error(w, "failed to get file content", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(html)
}

// @Summary Filter files by metadata
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Param metadata[] formData []string false "Metadata fields to filter on"
// @Param operator[] formData []string false "Filter operators (equals, contains, greater, less, in)"
// @Param value[] formData []string false "Filter values"
// @Param action[] formData []string false "Filter actions (include, exclude)"
// @Param logic formData string false "Logic operator for combining criteria (and, or)" default(and)
// @Success 200 {array} files.File
// @Router /api/files/filter [post]
func handleAPIFilterFiles(w http.ResponseWriter, r *http.Request) {
	logging.LogDebug("filter request received")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	logic := r.FormValue("logic")
	if logic == "" {
		logic = "and"
	}

	var criteria []files.FilterCriteria
	metadata := r.Form["metadata[]"]
	operators := r.Form["operator[]"]
	values := r.Form["value[]"]
	actions := r.Form["action[]"]

	maxLen := len(metadata)

	for i := range maxLen {
		if i < len(operators) && i < len(values) && metadata[i] != "" && operators[i] != "" {
			criteria = append(criteria, files.FilterCriteria{
				Metadata: metadata[i],
				Operator: operators[i],
				Value:    values[i],
				Action:   getFormValue(actions, i),
			})
		}
	}

	logging.LogDebug("built %d filter criteria: %+v", len(criteria), criteria)

	filteredFiles, err := files.FilterFilesByMetadata(criteria, logic)
	if err != nil {
		logging.LogError("failed to filter files: %v", err)
		http.Error(w, "failed to filter files", http.StatusInternalServerError)
		return
	}

	logging.LogDebug("filtered %d files", len(filteredFiles))

	var html strings.Builder
	html.WriteString(fmt.Sprintf("<p>found %d files</p>", len(filteredFiles)))
	html.WriteString("<ul>")
	for _, file := range filteredFiles {
		html.WriteString(fmt.Sprintf(`<li><a href="/files/%s">%s</a></li>`,
			strings.TrimPrefix(file.Path, "data/"),
			strings.TrimPrefix(file.Path, "data/")))
	}
	html.WriteString("</ul>")

	writeResponse(w, r, filteredFiles, html.String())
}

// @Summary Search files
// @Tags search
// @Param q query string true "Search query"
// @Produce json,html
// @Router /api/search [get]
func handleAPISearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "missing search query", http.StatusBadRequest)
		return
	}

	results, err := search.SearchFiles(query, 20)
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	var html strings.Builder
	html.WriteString("<ul>")
	for _, file := range results {
		html.WriteString(fmt.Sprintf(`<li><a href="/files/%s">%s</a></li>`,
			strings.TrimPrefix(file.Path, "data/"),
			strings.TrimPrefix(file.Path, "data/")))
	}
	html.WriteString("</ul>")

	writeResponse(w, r, results, html.String())
}

func getFormValue(slice []string, index int) string {
	if index < len(slice) {
		return slice[index]
	}
	return ""
}
