package server

import (
	"io"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
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
// @Description Snapshots the selected storages into a new backup set and trims expired sets. The default set (metadata, chat, kanban, notifications, config, search) is used when none are given; docs/media are optional and only included when named explicitly
// @Tags system
// @Accept application/x-www-form-urlencoded
// @Param storages formData []string false "Storage names to include (repeatable); omit for the default backup (excludes docs/media)"
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
// @Description Takes a fresh safety snapshot, restores a backup set onto disk, then restarts the app to apply it. If the set includes docs/media and a git remote is configured, the restored state is also force-pushed to it, overwriting anything there this device hasn't seen - confirm is then required to acknowledge that. Runs in the background - poll GET /api/jobs/{id} (returned in the response body/fragment) for completion.
// @Tags system
// @Accept application/x-www-form-urlencoded
// @Param name path string true "Backup set name"
// @Param confirm formData bool false "Required (true) when the set includes docs/media and a git remote is configured, acknowledging the restored state will be force-pushed to it"
// @Produce json,html
// @Success 200 {object} jobStorage.JobRecord
// @Router /api/system/backups/{name}/restore [post]
func handleAPIRestoreBackup(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	r.ParseForm()
	// The restore job only commits/force-pushes when the set's manifest includes docs/media (the
	// only storages that bypass git) - see job.restoreJob. Confirmation is only meaningful, so
	// only required, for that case.
	manifest, manifestErr := job.BackupManifest(name)
	touchesGit := manifestErr == nil && (slices.Contains(manifest, files.DocsStorageName) || slices.Contains(manifest, files.MediaStorageName))
	if touchesGit && git.RemoteEnabled() && r.FormValue("confirm") != "true" {
		msg := translation.SprintfForRequest(configmanager.GetLanguage(), "restore requires confirm=true: a git remote is configured, so the restored state will be force-pushed to it")
		notify.SetHeader(w, notify.LevelError, msg)
		writeResponse(w, r, nil, render.RenderStatusMessage(render.StatusError, msg))
		return
	}

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
