package server

import (
	"io"
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

// @Summary List the backup/restore history
// @Description Lists every backup created and restore applied, newest first
// @Tags system
// @Produce json,html
// @Success 200 {array} backup.LogEntry
// @Router /api/system/backups [get]
func handleAPIGetBackups(w http.ResponseWriter, r *http.Request) {
	entries, err := job.ListBackupLog()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to list backups"), http.StatusInternalServerError)
		return
	}
	writeResponse(w, r, entries, render.RenderBackupLog(entries))
}

// @Summary Create a backup
// @Description Snapshots the selected storages (metadata, chat, kanban, notifications, config, search - all of them when none are given) into a new backup set and trims expired sets
// @Tags system
// @Accept application/x-www-form-urlencoded
// @Param storages formData []string false "Storage names to include (repeatable); omit for a full backup"
// @Produce json,html
// @Success 200 {string} string "backup set name"
// @Router /api/system/backups [post]
func handleAPICreateBackup(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name, err := job.RunBackup(r.Form["storages"]...)
	if err != nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", err.Error()))
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", err.Error())))
		return
	}

	entries, err := job.ListBackupLog()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to list backups"), http.StatusInternalServerError)
		return
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "backup created"))
	writeResponse(w, r, map[string]string{"name": name}, render.RenderBackupLog(entries))
}

// @Summary Restore a backup
// @Description Takes a fresh safety snapshot, restores a backup set onto disk, then restarts the app to apply it. Runs in the background - poll GET /api/jobs/{id} (returned in the response body/fragment) for completion.
// @Tags system
// @Produce json,html
// @Param name path string true "Backup set name"
// @Success 200 {object} jobStorage.JobRecord
// @Router /api/system/backups/{name}/restore [post]
func handleAPIRestoreBackup(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	id, err := job.RunRestore(name)
	if err != nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", err.Error()))
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", err.Error())))
		return
	}

	lang := configmanager.GetLanguage()
	rec := &jobStorage.JobRecord{ID: id, Type: job.JobTypeRestore, Status: jobStorage.StatusRunning}
	writeResponse(w, r, rec, render.RenderJobStatus(lang, id, rec))
}

// @Summary Lock a backup
// @Description Marks a backup set to never be deleted by automatic rotation, until unlocked
// @Tags system
// @Produce json,html
// @Param name path string true "Backup set name"
// @Success 200 {array} backup.LogEntry
// @Router /api/system/backups/{name}/lock [post]
func handleAPILockBackup(w http.ResponseWriter, r *http.Request) {
	handleBackupLockToggle(w, r, job.LockBackup)
}

// @Summary Unlock a backup
// @Description Removes a backup set's protection from automatic rotation
// @Tags system
// @Produce json,html
// @Param name path string true "Backup set name"
// @Success 200 {array} backup.LogEntry
// @Router /api/system/backups/{name}/lock [delete]
func handleAPIUnlockBackup(w http.ResponseWriter, r *http.Request) {
	handleBackupLockToggle(w, r, job.UnlockBackup)
}

func handleBackupLockToggle(w http.ResponseWriter, r *http.Request, apply func(string) error) {
	name := chi.URLParam(r, "name")

	if err := apply(name); err != nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", err.Error()))
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", err.Error())))
		return
	}

	entries, err := job.ListBackupLog()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to list backups"), http.StatusInternalServerError)
		return
	}
	writeResponse(w, r, entries, render.RenderBackupLog(entries))
}

// @Summary Download a backup
// @Description Downloads a backup set's raw .tar.gz archive
// @Tags system
// @Produce application/gzip
// @Param name path string true "Backup set name"
// @Success 200 {file} file
// @Router /api/system/backups/{name}/download [get]
func handleAPIDownloadBackup(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	rc, err := job.OpenBackup(name)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, translation.SprintfForRequest(configmanager.GetLanguage(), "%s", err.Error()))
		return
	}
	defer rc.Close()

	setAttachmentFilename(w, name+".tar.gz")
	w.Header().Set("Content-Type", "application/gzip")
	if _, err := io.Copy(w, rc); err != nil {
		logging.LogWarning(logging.KeyApp, "backup download: failed to stream %s: %v", name, err)
	}
}
