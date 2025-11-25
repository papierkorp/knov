package server

import (
	"net/http"
	"strconv"

	"knov/internal/configmanager"
	"knov/internal/git"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get recently changed files
// @Tags git
// @Produce json,html
// @Router /api/git/latestchanges [get]
func handleAPIGetRecentlyChanged(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count := 100 // default
	if countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil {
			count = c
		}
	}

	files, err := git.GetRecentlyChangedFiles(count)
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get recent files"), http.StatusInternalServerError)
		return
	}

	html := render.RenderGitHistoryFileList(files)
	writeResponse(w, r, files, html)
}
