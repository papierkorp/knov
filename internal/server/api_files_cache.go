// Package server ..
package server

import (
	"fmt"
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get file tree overview
// @Description Returns all files as an indented folder tree structure
// @Tags files
// @Produce json,html
// @Router /api/files/tree [get]
func handleAPIGetFileTree(w http.ResponseWriter, r *http.Request) {
	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get files"), http.StatusInternalServerError)
		return
	}
	allFiles = files.FilterByVisibility(allFiles)
	tree := files.BuildFileTree(allFiles)
	html := render.RenderTreeOverview(tree, r.URL.Query().Get("actions") == "true")
	writeResponse(w, r, allFiles, html)
}

// @Summary Get all files
// @Tags files
// @Param format query string false "Response format (options for HTML select options)"
// @Produce json,html
// @Router /api/files/list [get]
func handleAPIGetAllFiles(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")

	if format == "options" {
		cachedFilePaths, err := files.GetAllFilePathsFromCache()
		if err != nil {
			logging.LogError("failed to get cached file paths: %v", err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get files"), http.StatusInternalServerError)
			return
		}
		html := render.RenderFilesOptionsFromPaths(cachedFilePaths)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
		return
	}

	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get files"), http.StatusInternalServerError)
		return
	}

	allFiles = files.FilterByVisibility(allFiles)

	if format == "datalist" {
		html := render.RenderFilesDatalist(allFiles)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
		return
	}

	html := render.RenderFilesList(allFiles, r.URL.Query().Get("actions") == "true")
	writeResponse(w, r, allFiles, html)
}
