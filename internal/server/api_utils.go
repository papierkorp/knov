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
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
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

// viewedFile returns the docs-relative path of the file shown on the page an htmx request came
// from (read from the HX-Current-URL header), or "".
func viewedFile(r *http.Request) string {
	return pathutils.FileFromURL(r.Header.Get("HX-Current-URL"))
}

// metaPathParam reads the param name naming an existing file: its metadata path ("docs/..." or
// "media/..."). An empty value is returned as is for the handler's own missing-param answer, any
// other value without the prefix is answered with 400 and ok is false.
func metaPathParam(w http.ResponseWriter, r *http.Request, name string) (path string, ok bool) {
	path = r.FormValue(name)
	if path != "" && !pathutils.IsMetaPath(path) {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "%s must start with docs/ or media/", name))
		return "", false
	}
	return path, true
}

// linkKindParam reads the "link" query param of the autocomplete apis - the link syntax a
// suggestion is inserted into ("wiki" or "markdown"); ok is false without one.
func linkKindParam(r *http.Request) (kind parser.LinkKind, ok bool) {
	switch r.URL.Query().Get("link") {
	case "wiki":
		return parser.LinkWiki, true
	case "markdown":
		return parser.LinkMarkdown, true
	}
	return kind, false
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
// handler otherwise repeats.
func respondJobStarted(w http.ResponseWriter, r *http.Request, id, jobType string) {
	rec, html := renderRunningJob(configmanager.GetLanguage(), id, jobType)
	writeResponse(w, r, rec, html)
}

// renderRunningJob builds the record and polling spinner of the running async job id, e.g. for
// a page that shows a job still running after a reload. Delete-folder is the lone browse-tree
// caller, whose hx-target is a tree row rather than a bare span, so its span is wrapped in <li>.
func renderRunningJob(lang, id, jobType string) (*jobStorage.JobRecord, string) {
	rec := &jobStorage.JobRecord{ID: id, Type: jobType, Status: jobStorage.StatusRunning}
	cancellable := job.IsCancellable(jobType)
	progress := job.GetProgress(id)
	if jobType == job.JobTypeDeleteFolder {
		return rec, render.RenderJobStatusListItem(lang, id, rec, cancellable, progress)
	}
	return rec, render.RenderJobStatus(lang, id, rec, cancellable, progress)
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

// writeNewPathError answers 400 if err is pathutils.ErrInvalidName and reports whether it did.
func writeNewPathError(w http.ResponseWriter, r *http.Request, err error) bool {
	message, ok := newPathMessage(err)
	if ok {
		writeAPIError(w, r, http.StatusBadRequest, message)
	}
	return ok
}

// newPathMessage is the translated response text for pathutils.ErrInvalidName, ok is false for
// any other error.
func newPathMessage(err error) (message string, ok bool) {
	switch {
	case errors.Is(err, pathutils.ErrInvalidName):
		return translation.SprintfForRequest(configmanager.GetLanguage(), "file and folder names can't contain %s or start or end with a space", `# ? | [ ] \`), true
	}
	return "", false
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
	case errors.Is(err, pathutils.ErrInvalidName):
		message, _ := newPathMessage(err)
		respond(http.StatusBadRequest, message)
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
