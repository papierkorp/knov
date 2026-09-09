// Package server - API utility functions
package server

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/job"
	"knov/internal/jobStorage"
	"knov/internal/logging"
	"knov/internal/server/render"
)

func writeResponse(w http.ResponseWriter, r *http.Request, jsonData any, htmlData string) {
	acceptHeader := r.Header.Get("Accept")

	if strings.Contains(acceptHeader, "text/html") || strings.Contains(acceptHeader, "*/*") {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlData))
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonData)
	}
}

// setAttachmentFilename sets a Content-Disposition header for filename,
// properly quoting/escaping it per RFC 6266/2231 (via mime.FormatMediaType)
// instead of splicing it into the header unquoted — a filename containing a
// space or other token-breaking character would otherwise produce a
// malformed header.
func setAttachmentFilename(w http.ResponseWriter, filename string) {
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
}

// respondJobStarted writes the initial htmx polling-spinner response for a just-started async
// job (job.StartAsync), collapsing the identical record-build + render block every StartAsync
// handler otherwise repeats. Delete-folder is the lone browse-tree caller, whose hx-target is a
// tree row rather than a bare span, so its span is wrapped in <li>.
func respondJobStarted(w http.ResponseWriter, r *http.Request, id, jobType string) {
	lang := configmanager.GetLanguage()
	rec := &jobStorage.JobRecord{ID: id, Type: jobType, Status: jobStorage.StatusRunning}
	cancellable := job.IsCancellable(jobType)
	progress := job.GetProgress(id)
	if jobType == job.JobTypeDeleteFolder {
		writeResponse(w, r, rec, render.RenderJobStatusListItem(lang, id, rec, cancellable, progress))
		return
	}
	writeResponse(w, r, rec, render.RenderJobStatus(lang, id, rec, cancellable, progress))
}

// writeAPIError writes a status-coded HTML error response, replacing the
// repeated header/status/write block previously duplicated across the file
// rename/move/delete handlers.
func writeAPIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)
	w.Write([]byte(render.RenderStatusMessage(render.StatusError, message)))
}

// moveErrorMessages holds a call site's translated response text for each outcome
// handleMoveError distinguishes, so the classification logic can be shared without dictating
// wording that differs (deliberately) between the rename/media-rename/set-path handlers.
type moveErrorMessages struct {
	sourceMissing, targetExists, moveFailed string
}

// handleMoveError classifies an error from files.MoveFileNoRefresh/MoveMediaFileNoRefresh and
// reports it, replacing the same 4-way errors.Is switch previously duplicated across every move
// handler. respond is called with the response status/message for a fatal outcome (source
// missing, target exists, or any other failure) - the caller returns immediately afterward. A
// nil error is a no-op; the non-fatal ErrLinkUpdateFailed (the physical move already succeeded,
// only the secondary link-rewrite step failed) is only logged, matching every call site's
// existing "don't fail the operation for this" handling. stop reports whether the caller should
// return.
func handleMoveError(err error, context, oldPath, newPath string, msgs moveErrorMessages, respond func(status int, message string)) (stop bool) {
	switch {
	case err == nil:
		return false
	case errors.Is(err, files.ErrMoveSourceMissing):
		respond(http.StatusNotFound, msgs.sourceMissing)
		return true
	case errors.Is(err, files.ErrMoveTargetExists):
		respond(http.StatusConflict, msgs.targetExists)
		return true
	case errors.Is(err, files.ErrLinkUpdateFailed):
		logging.LogWarning(logging.KeyApp, "%s: failed to update links %s -> %s: %v", context, oldPath, newPath, err)
		return false
	default:
		logging.LogError(logging.KeyApp, "%s: failed to move %s -> %s: %v", context, oldPath, newPath, err)
		respond(http.StatusInternalServerError, msgs.moveFailed)
		return true
	}
}
