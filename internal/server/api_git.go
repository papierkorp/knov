package server

import (
	"net/http"
	"strconv"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/job"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get recently changed files
// @Description Returns recently changed files. When q is set, searches git history by filename instead of returning latest.
// @Tags git
// @Param count query int false "Number of results (default 50)"
// @Param offset query int false "Offset for pagination (default 0)"
// @Param q query string false "Search query — filters by filename in git history"
// @Param collection query string false "Filter by collection"
// @Param folder query string false "Filter by folder, recursive (includes subfolders)"
// @Produce json,html
// @Router /api/git/latestchanges [get]
func handleAPIGetRecentlyChanged(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count := 50
	if countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil {
			count = c
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	query := r.URL.Query().Get("q")
	collection := r.URL.Query().Get("collection")
	folder := r.URL.Query().Get("folder")

	// search mode — git title search, no pagination
	if query != "" {
		results, err := git.SearchGitByTitle(query, count, false)
		if err != nil {
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to search git history"), http.StatusInternalServerError)
			return
		}
		html := render.RenderGitHistoryFileList(results, "", "", 0, false)
		writeResponse(w, r, results, html)
		return
	}

	allFiles, err := git.GetRecentlyChangedFiles(count, offset)
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get recent files"), http.StatusInternalServerError)
		return
	}

	unfilteredCount := len(allFiles)
	if collection != "" || folder != "" {
		var filtered []git.GitHistoryFile
		for _, f := range allFiles {
			meta, err := files.MetaDataGet(pathutils.ToWithPrefix(f.Path))
			if err != nil || meta == nil {
				continue
			}
			if collection != "" && meta.Collection != collection {
				continue
			}
			if folder != "" && !pathutils.FolderContains(strings.Join(meta.Folders, "/"), folder) {
				continue
			}
			filtered = append(filtered, f)
		}
		allFiles = filtered
	}

	hasMore := unfilteredCount == count
	html := render.RenderGitHistoryFileList(allFiles, collection, folder, offset+count, hasMore)
	writeResponse(w, r, allFiles, html)
}

// @Summary Push to remote
// @Description Manually trigger a git push to the configured remote
// @Tags git
// @Produce json,html
// @Success 200 {string} string "push triggered"
// @Router /api/git/push [post]
func handleAPIGitPush(w http.ResponseWriter, r *http.Request) {
	if err := job.RunGitPush(); err != nil {
		level := notify.LevelError
		if configmanager.GetGitRemote() == "" {
			level = notify.LevelWarning
		}
		notify.SetHeader(w, level, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error())))
		return
	}
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "push triggered"))
	writeResponse(w, r, map[string]string{"status": "push triggered"}, "")
}

// @Summary Pull from remote
// @Description Manually trigger a git pull --rebase from the configured remote
// @Tags git
// @Produce json,html
// @Success 200 {string} string "pull completed"
// @Router /api/git/pull [post]
func handleAPIGitPull(w http.ResponseWriter, r *http.Request) {
	if err := job.RunGitPull(); err != nil {
		level := notify.LevelError
		if configmanager.GetGitRemote() == "" {
			level = notify.LevelWarning
		}
		notify.SetHeader(w, level, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error()))
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), err.Error())))
		return
	}
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "pull completed"))
	writeResponse(w, r, map[string]string{"status": "pull completed"}, "")
}

// @Summary Test git SSH auth (debug)
// @Description Tests SSH/HTTPS authentication against the configured remote and logs the result
// @Tags git
// @Produce json,html
// @Success 200 {string} string "auth test result"
// @Router /api/git/test-auth [post]
func handleAPIGitTestAuth(w http.ResponseWriter, r *http.Request) {
	remote := configmanager.GetGitRemote()
	if remote == "" {
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError,
			translation.SprintfForRequest(configmanager.GetLanguage(), "no remote configured")))
		return
	}

	result, err := git.TestAuth()
	if err != nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "auth test failed"))
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError, err.Error()))
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "auth test passed"))
	writeResponse(w, r, map[string]string{"result": result}, render.RenderStatusMessage(render.StatusOK, result))
}
