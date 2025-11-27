// Package server - API handlers for file version operations
package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get file versions
// @Description List all versions of a file or get specific version content
// @Tags files
// @Param filepath path string true "File path"
// @Param commit query string false "Specific commit (current, previous, or commit hash)"
// @Produce json,html
// @Success 200 "File versions or specific version content"
// @Router /api/files/versions/{filepath} [get]
func handleAPIGetFileVersions(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/versions/")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(configmanager.GetAppConfig().DataPath, filePath)
	commit := r.URL.Query().Get("commit")

	// if no commit specified, return list of all versions
	if commit == "" {
		versions, err := git.GetFileHistory(fullPath)
		if err != nil {
			logging.LogDebug("failed to get file history for %s: %v", filePath, err)
			// if this is a sidebar request, return a friendlier message
			if r.URL.Query().Get("sidebar") == "true" {
				html := `<div class="no-versions">no git history available</div>`
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte(html))
				return
			}
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file history"), http.StatusInternalServerError)
			return
		}

		// check if this is a sidebar request
		if r.URL.Query().Get("sidebar") == "true" {
			html := render.RenderFileVersionsSidebar(versions, filePath)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
			return
		}

		html := render.RenderFileVersionsList(versions, filePath)
		writeResponse(w, r, versions, html)
		return
	}

	// handle special commit values
	switch commit {
	case "current":
		// get current file content
		content, err := os.ReadFile(fullPath)
		if err != nil {
			logging.LogError("failed to read current file %s: %v", filePath, err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusInternalServerError)
			return
		}
		html := render.RenderFileAtVersion(string(content), filePath, "current", "current", translation.SprintfForRequest(configmanager.GetLanguage(), "current version"))
		writeResponse(w, r, string(content), html)
		return

	case "previous":
		// get file history and get previous commit
		versions, err := git.GetFileHistory(fullPath)
		if err != nil || len(versions) < 2 {
			logging.LogError("failed to get previous version for %s: %v", filePath, err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "no previous version available"), http.StatusNotFound)
			return
		}
		commit = versions[1].Commit // second item is previous
	}

	// get file content at specific commit
	content, err := git.GetFileAtCommit(fullPath, commit)
	if err != nil {
		logging.LogDebug("failed to get file %s at commit %s: %v", filePath, commit, err)
		// check if this is a sidebar request and return a friendlier message
		if r.URL.Query().Get("sidebar") == "true" {
			html := `<div class="version-error">` + translation.SprintfForRequest(configmanager.GetLanguage(), "version no longer available") + `</div>`
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
			return
		}
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "version no longer available"), http.StatusNotFound)
		return
	}

	// get commit details for display
	date, message, err := git.GetCommitDetails(commit)
	if err != nil {
		// fallback if commit details can't be retrieved
		date = "unknown"
		message = "commit details unavailable"
		logging.LogDebug("failed to get commit details for %s: %v", commit, err)
	}

	html := render.RenderFileAtVersion(content, filePath, commit, date, message)
	writeResponse(w, r, content, html)
}

// @Summary Get file version diff
// @Description Compare two versions of a file
// @Tags files
// @Param filepath path string true "File path"
// @Param from query string true "From commit hash or 'current'"
// @Param to query string true "To commit hash or 'current'"
// @Produce html
// @Success 200 "File diff content"
// @Router /api/files/versions/diff/{filepath} [get]
func handleAPIGetFileVersionDiff(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/versions/diff/")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	fromCommit := r.URL.Query().Get("from")
	toCommit := r.URL.Query().Get("to")

	if fromCommit == "" || toCommit == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing from or to parameters"), http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(configmanager.GetAppConfig().DataPath, filePath)

	// handle special commit values
	if fromCommit == "current" {
		currentCommit, err := git.GetCurrentCommit()
		if err != nil {
			logging.LogError("failed to get current commit: %v", err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get current commit"), http.StatusInternalServerError)
			return
		}
		fromCommit = currentCommit
	}

	if toCommit == "current" {
		currentCommit, err := git.GetCurrentCommit()
		if err != nil {
			logging.LogError("failed to get current commit: %v", err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get current commit"), http.StatusInternalServerError)
			return
		}
		toCommit = currentCommit
	}

	diff, err := git.GetFileDiff(fullPath, fromCommit, toCommit)
	if err != nil {
		logging.LogError("failed to get diff for %s between %s and %s: %v", filePath, fromCommit, toCommit, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get file diff"), http.StatusInternalServerError)
		return
	}

	html := render.RenderFileDiff(diff, filePath, fromCommit, toCommit)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Restore file version
// @Description Restore a file to a specific version
// @Tags files
// @Accept application/x-www-form-urlencoded
// @Param filepath path string true "File path"
// @Param commit formData string true "Commit hash to restore to"
// @Produce json,html
// @Success 200 "File restored successfully"
// @Router /api/files/versions/restore/{filepath} [post]
func handleAPIRestoreFileVersion(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/versions/restore/")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	commit := r.FormValue("commit")
	if commit == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing commit parameter"), http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(configmanager.GetAppConfig().DataPath, filePath)

	err := git.RestoreFileToCommit(fullPath, commit)
	if err != nil {
		logging.LogError("failed to restore file %s to commit %s: %v", filePath, commit, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to restore file"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("restored file %s to commit %s", filePath, commit)

	// return success message
	html := fmt.Sprintf(`<div class="success-message">%s</div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "file restored to commit %s and logged in git history", commit))
	w.Header().Set("HX-Refresh", "true") // refresh the page to show updated content
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
