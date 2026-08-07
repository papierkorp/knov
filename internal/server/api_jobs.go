// Package server ..
package server

import (
	"net/http"
	"strings"

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
	id := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
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
	case rec.Status == jobStorage.StatusDone && rec.Type == "bulk-delete-files":
		// bulk-delete-files' triggering page needs a full reload to drop the deleted files
		// from its filtered list, exactly like the old synchronous handler did - so redirect
		// instead of an in-page toast, whose HX-Trigger would be lost on navigation anyway.
		notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(lang, "files deleted"))
		if groupType, _, perr := job.ParseBulkDeleteArgs(rec.Args); perr == nil {
			w.Header().Set("HX-Redirect", "/browse/"+groupType)
		}
	case rec.Status == jobStorage.StatusDone:
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(lang, "folder deleted"))
	case rec.Status != jobStorage.StatusRunning:
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(lang, "job failed: %s", rec.Error))
	}

	writeResponse(w, r, rec, render.RenderJobStatus(lang, id, rec))
}
