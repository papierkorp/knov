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
	"knov/internal/server/notify"
	"knov/internal/server/render"
)

// wantsHTML reports whether the response body should be HTML rather than JSON.
// It is the single content-negotiation rule shared by writeResponse and
// writeAPIError so success and error responses always agree on the format.
// An explicit "application/json" in Accept always wins (API clients that also
// send a trailing "*/*" still get JSON); otherwise "text/html" or a bare "*/*"
// (htmx / browsers) selects HTML. A missing Accept header defaults to JSON.
func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return false
	}
	return strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

func writeResponse(w http.ResponseWriter, r *http.Request, jsonData any, htmlData string) {
	if wantsHTML(r) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlData))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jsonData)
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

// writeAPIError writes an honest status-coded error response whose body honours the
// request Accept header the same way writeResponse does - an inline status-message
// span for htmx/browser clients (htmx 4 swaps 4xx/5xx bodies by default, so the
// error still shows inline), a JSON error object for application/json clients.
//
// It also fires an error toast + notification-log entry via notify.SetHeader;
// KNOV_NOTIFY_MIN_LEVEL mutes the toast (the log entry always persists).
func writeAPIError(w http.ResponseWriter, r *http.Request, status int, message string) {
	notify.SetHeader(w, notify.LevelError, message)
	if wantsHTML(r) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(status)
		w.Write([]byte(render.RenderStatusMessage(render.StatusError, message)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
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
