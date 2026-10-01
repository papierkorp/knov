// Package server ..
package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"knov/internal/configmanager"
	"knov/internal/export"
	"knov/internal/job"
	"knov/internal/jobStorage"
	"knov/internal/logging"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get async job status
// @Description Polled by htmx while an async job (started via a delete-folder or bulk-delete
// @Description request) runs in the background; returns a small status fragment - a
// @Description self-polling spinner while running, empty once done, or an inline error message.
// @Tags jobs
// @Produce json,html
// @Param id path string true "Job id"
// @Success 200 {object} jobStorage.JobRecord
// @Failure 404 {object} string "job not found"
// @Router /api/jobs/{id} [get]
func handleAPIGetJobStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing job id"))
		return
	}

	rec, err := jobStorage.Get(id)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to load job status %s: %v", id, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to load job status"))
		return
	}
	if rec == nil {
		writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "job not found"))
		return
	}

	lang := configmanager.GetLanguage()
	switch {
	case rec.Status == jobStorage.StatusDone && rec.Type == job.JobTypeBulkDeleteFiles:
		// bulk-delete-files' triggering page needs a full reload to drop the deleted files
		// from its filtered list, exactly like the old synchronous handler did - so redirect
		// instead of an in-page toast, whose HX-Trigger would be lost on navigation anyway.
		groupType, _, perr := job.ParseBulkDeleteArgs(rec.Args)
		if perr != nil {
			logging.LogError(logging.KeyApp, "failed to parse bulk-delete-files args for job %s: %v", id, perr)
			notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(lang, "job failed: %s", perr.Error()))
			break
		}
		notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(lang, "files deleted"))
		w.Header().Set("HX-Redirect", "/browse/"+groupType)
	case rec.Status == jobStorage.StatusDone && rec.Type == job.JobTypeDeleteFolder:
		// same reasoning as bulk-delete-files above: the folder's row was already swapped out
		// for this spinner, so a full reload is needed to make the browse tree reflect the
		// removal - an in-page toast alone would leave an empty <li> behind.
		notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(lang, "folder deleted"))
		w.Header().Set("HX-Redirect", "/browse")
	case rec.Status == jobStorage.StatusDone && rec.Type == job.JobTypeFullRebuild:
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(lang, "metadata rebuilt successfully"))
	case rec.Status == jobStorage.StatusDone && rec.Type == job.JobTypeRestore:
		// the app restarts itself a moment after this - see restoreJob.Run - so there's nothing
		// to redirect to; a toast is all a client still connected at that instant will show.
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(lang, "backup restored, application restarting..."))
	case rec.Status == jobStorage.StatusDone && rec.Type == job.JobTypeFileSync:
		// no toast - the kanban board's own container listens for this event and refetches
		// itself (see #view-kanban-board-wrap's hx-trigger in kanban.gohtml), which is
		// feedback enough.
		w.Header().Set("HX-Trigger", "kanban-sync-done")
	case rec.Status == jobStorage.StatusDone && rec.Type == job.JobTypeExport:
		// no auto-download via HX-Redirect - it cancels the swap, so the polling spinner would
		// stay and re-trigger the download every second. the download link is appended below.
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(lang, "export finished"))
	case rec.Status == jobStorage.StatusDone:
		// generic fallback for any future StartAsync job type not special-cased above.
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(lang, "done"))
	case rec.Status == jobStorage.StatusCanceled && rec.Type == job.JobTypeBulkDeleteFiles:
		// same reload reasoning as the StatusDone case above - a cancel still leaves whatever
		// was deleted before it took effect, so the filtered list needs to drop those too.
		groupType, _, perr := job.ParseBulkDeleteArgs(rec.Args)
		if perr != nil {
			logging.LogError(logging.KeyApp, "failed to parse bulk-delete-files args for job %s: %v", id, perr)
			notify.SetHeader(w, notify.LevelWarning, translation.SprintfForRequest(lang, "job canceled"))
			break
		}
		notify.SetFlash(notify.LevelWarning, translation.SprintfForRequest(lang, "deletion canceled"))
		w.Header().Set("HX-Redirect", "/browse/"+groupType)
	case rec.Status == jobStorage.StatusCanceled && rec.Type == job.JobTypeDeleteFolder:
		notify.SetFlash(notify.LevelWarning, translation.SprintfForRequest(lang, "deletion canceled"))
		w.Header().Set("HX-Redirect", "/browse")
	case rec.Status == jobStorage.StatusCanceled:
		notify.SetHeader(w, notify.LevelWarning, translation.SprintfForRequest(lang, "job canceled"))
	case rec.Status != jobStorage.StatusRunning:
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(lang, "job failed: %s", rec.Error))
	}

	html := render.RenderJobStatus(lang, id, rec, job.IsCancellable(rec.Type), job.GetProgress(id))
	// a finished, canceled or failed export still has an archive (new or previous) to download
	if rec.Type == job.JobTypeExport && rec.Status != jobStorage.StatusRunning && export.Available() {
		html += render.RenderExportDone(lang)
	}
	writeResponse(w, r, rec, html)
}

// @Summary Cancel a running async job
// @Description Requests cancellation of a running async job (a delete-folder, bulk-delete or
// @Description metadata-full-rebuild run) by canceling its context. Cancellation is cooperative - the
// @Description job only stops at its next checkpoint, so the response still reflects "running";
// @Description poll GET /api/jobs/{id} for the eventual "canceled" status. Only job types the
// @Description jobs UI shows a cancel button for actually honor it - see job.IsCancellable.
// @Tags jobs
// @Produce json,html
// @Param id path string true "Job id"
// @Success 200 {object} jobStorage.JobRecord
// @Failure 404 {object} string "job not running"
// @Router /api/jobs/{id} [delete]
func handleAPIDeleteJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing job id"))
		return
	}

	if err := job.CancelAsync(id); err != nil {
		if errors.Is(err, job.ErrNotRunning) {
			writeAPIError(w, r, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "job not running"))
			return
		}
		logging.LogError(logging.KeyApp, "failed to cancel job %s: %v", id, err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to cancel job"))
		return
	}

	handleAPIGetJobStatus(w, r)
}
