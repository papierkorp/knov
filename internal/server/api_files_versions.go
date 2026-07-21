// Package server - API handlers for file version operations
package server

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get file versions
// @Description List all versions of a file or get specific version content
// @Tags files
// @Param filepath path string true "File path"
// @Param commit query string false "Specific commit (current, previous, or commit hash)"
// @Param output query string false "Output format: full, sidebar, compact, content (default: full)"
// @Produce json,html
// @Success 200 "File versions or specific version content"
// @Router /api/files/versions/{filepath} [get]
func handleAPIGetFileVersions(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/api/files/versions/")
	if filePath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing filepath parameter"), http.StatusBadRequest)
		return
	}

	fullPath := pathutils.ToFullPath(filePath)
	commit := r.URL.Query().Get("commit")
	output := r.URL.Query().Get("output")
	if output == "" {
		output = "full"
	}

	if commit == "" {
		versions, err := git.GetFileHistory(fullPath)
		if err != nil {
			logging.LogDebug("failed to get file history for %s: %v", filePath, err)
			html := `<div class="no-versions">` + translation.SprintfForRequest(configmanager.GetLanguage(), "no git history available") + `</div>`
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
			return
		}
		html := render.RenderFileVersionsList(versions, filePath, output)
		writeResponse(w, r, versions, html)
		return
	}

	switch commit {
	case "current":
		content, err := os.ReadFile(fullPath)
		if err != nil {
			logging.LogError("failed to read current file %s: %v", filePath, err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusInternalServerError)
			return
		}
		html := render.RenderFileAtVersion(string(content), filePath, "current", "current", translation.SprintfForRequest(configmanager.GetLanguage(), "current version"), output)
		writeResponse(w, r, string(content), html)
		return

	case "previous":
		versions, err := git.GetFileHistory(fullPath)
		if err != nil || len(versions) < 2 {
			logging.LogError("failed to get previous version for %s: %v", filePath, err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "no previous version available"), http.StatusNotFound)
			return
		}
		commit = versions[1].Commit
	}

	content, err := git.GetFileAtCommit(fullPath, commit)
	if err != nil {
		logging.LogDebug("failed to get file %s at commit %s: %v", filePath, commit, err)
		html := `<div class="version-error">` + translation.SprintfForRequest(configmanager.GetLanguage(), "version no longer available") + `</div>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}

	commitTime, message, err := git.GetCommitDetails(commit)
	date := configmanager.FormatDateTime(commitTime)
	if err != nil {
		date = "unknown"
		message = "commit details unavailable"
		logging.LogDebug("failed to get commit details for %s: %v", commit, err)
	}

	html := render.RenderFileAtVersion(content, filePath, commit, date, message, output)
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

	fullPath := pathutils.ToFullPath(filePath)

	if fromCommit == "current" {
		currentCommit, err := git.GetCurrentCommit()
		if err != nil {
			logging.LogError("failed to get current commit: %v", err)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div class="version-error">%s</div>`,
				translation.SprintfForRequest(configmanager.GetLanguage(), "diff not available"))
			return
		}
		fromCommit = currentCommit
	}

	if toCommit == "current" {
		currentCommit, err := git.GetCurrentCommit()
		if err != nil {
			logging.LogError("failed to get current commit: %v", err)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div class="version-error">%s</div>`,
				translation.SprintfForRequest(configmanager.GetLanguage(), "diff not available"))
			return
		}
		toCommit = currentCommit
	}

	if toCommit == "previous" {
		versions, err := git.GetFileHistory(fullPath)
		if err != nil || len(versions) < 2 {
			logging.LogError("failed to get previous commit for %s: %v", filePath, err)
			http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to get previous commit"), http.StatusInternalServerError)
			return
		}
		for i, v := range versions {
			if v.Commit == fromCommit && i+1 < len(versions) {
				toCommit = versions[i+1].Commit
				break
			}
		}
		if toCommit == "previous" {
			if len(versions) > 1 {
				toCommit = versions[1].Commit
			} else {
				http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "no previous version found"), http.StatusNotFound)
				return
			}
		}
	}

	diff, oldCommit, newCommit, err := git.GetFileDiff(fullPath, fromCommit, toCommit)
	if err != nil {
		logging.LogError("failed to get diff for %s between %s and %s: %v", filePath, fromCommit, toCommit, err)
		// return soft HTML error so htmx does not treat this as a hard failure
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div class="version-error">%s</div>`,
			translation.SprintfForRequest(configmanager.GetLanguage(), "diff not available"))
		return
	}

	currentCommit := ""
	if versions, err := git.GetFileHistory(fullPath); err == nil && len(versions) > 0 {
		currentCommit = versions[0].Commit
	}

	before := buildFileDiffVersion(fullPath, oldCommit, currentCommit)
	after := buildFileDiffVersion(fullPath, newCommit, currentCommit)

	html := render.RenderFileDiff(diff, filePath, before, after)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// buildFileDiffVersion loads the display metadata and full content for one
// side of a file diff at the given commit.
func buildFileDiffVersion(fullPath, commit, currentCommit string) render.FileDiffVersion {
	date, message, err := git.GetCommitDetails(commit)
	dateStr := "unknown"
	if err == nil {
		dateStr = configmanager.FormatDateTime(date)
	}
	content, err := git.GetFileAtCommit(fullPath, commit)
	if err != nil {
		content = translation.SprintfForRequest(configmanager.GetLanguage(), "version no longer available")
	}
	// currentCommit comes from GetFileHistory as a short (7-char) hash while
	// commit here is always the full hash, so compare by prefix
	isCurrent := currentCommit != "" && strings.HasPrefix(commit, currentCommit)
	return render.FileDiffVersion{Commit: commit, Date: dateStr, Message: message, Content: content, IsCurrent: isCurrent}
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

	fullPath := pathutils.ToFullPath(filePath)

	if err := git.RestoreFileToCommit(fullPath, commit); err != nil {
		logging.LogError("failed to restore file %s to commit %s: %v", filePath, commit, err)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to restore file"))
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to restore file"), http.StatusInternalServerError)
		return
	}

	logging.LogInfo("restored file %s to commit %s", filePath, commit)
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "file restored to version %s", commit))
	w.Header().Set("HX-Refresh", "true")
	writeResponse(w, r, map[string]string{"status": "restored", "commit": commit}, "")
}
