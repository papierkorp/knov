package server

import (
	"net/http"
	"strconv"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/pathutils"
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

	collection := r.URL.Query().Get("collection")

	allFiles, err := git.GetRecentlyChangedFiles(count)
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get recent files"), http.StatusInternalServerError)
		return
	}

	if collection == "" {
		html := render.RenderGitHistoryFileList(allFiles)
		writeResponse(w, r, allFiles, html)
		return
	}

	// filter by collection
	var filtered []git.GitHistoryFile
	for _, f := range allFiles {
		meta, err := files.MetaDataGet(pathutils.ToWithPrefix(f.Path))
		if err != nil || meta == nil {
			continue
		}
		if meta.Collection == collection {
			filtered = append(filtered, f)
		}
	}

	html := render.RenderGitHistoryFileList(filtered)
	writeResponse(w, r, filtered, html)
}
