package server

import (
	"net/http"
	"strconv"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
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

// @Summary Push to remote
// @Description Manually trigger a git push to the configured remote
// @Tags git
// @Produce json,html
// @Success 200 {string} string "push triggered"
// @Router /api/git/push [post]
func handleAPIGitPush(w http.ResponseWriter, r *http.Request) {
	if configmanager.GetGitRemote() == "" {
		notify.SetHeader(w, notify.LevelWarning, translation.SprintfForRequest(configmanager.GetLanguage(), "no remote configured"))
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "no remote configured")))
		return
	}
	git.Push()
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
	if configmanager.GetGitRemote() == "" {
		notify.SetHeader(w, notify.LevelWarning, translation.SprintfForRequest(configmanager.GetLanguage(), "no remote configured"))
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "no remote configured")))
		return
	}
	if err := git.PullRebase(); err != nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "pull failed: %s", err.Error()))
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "pull failed")))
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
