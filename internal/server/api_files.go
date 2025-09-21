// Package server ..
package server

import (
	"fmt"
	"net/http"
	"strings"

	"knov/internal/files"
	"knov/internal/logging"
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

	criteria, logic, err := files.ParseFilterCriteria(r)
	if err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
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
