// Package server ..
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"knov/internal/configmanager"
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
// @Produce html
// @Param id path string true "Job id"
// @Success 200 {object} jobStorage.JobRecord
// @Failure 404 {object} string "job not found"
// @Router /api/jobs/{id} [get]
func handleAPIGetJobStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing job id"))
		return
	}

	rec, err := jobStorage.Get(id)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to load job status %s: %v", id, err)
		writeAPIError(w, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to load job status"))
		return
	}
	if rec == nil {
		writeAPIError(w, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "job not found"))
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
	case rec.Status == jobStorage.StatusDone:
		// generic fallback for any future StartAsync job type not special-cased above.
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(lang, "done"))
	case rec.Status != jobStorage.StatusRunning:
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(lang, "job failed: %s", rec.Error))
	}

	writeResponse(w, r, rec, render.RenderJobStatus(lang, id, rec))
}
