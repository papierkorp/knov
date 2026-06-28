package server

import (
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/job"
	"knov/internal/server/notify"
	"knov/internal/translation"
)

// @Summary Run cronjob
// @Description Manually triggers the cronjob execution (file processing and search indexing)
// @Tags cronjob
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Success 200 {object} string "{"status":"ok","message":"cronjob executed successfully"}"
// @Failure 409 {object} string "already running"
// @Router /api/cronjob [post]
func handleAPIRunCronjob(w http.ResponseWriter, r *http.Request) {
	if err := job.RunAsync(); err != nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "cronjob is already running"))
		http.Error(w, "cronjob is already running", http.StatusConflict)
		return
	}
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "cronjob started"))
	writeResponse(w, r, map[string]string{"status": "ok", "message": "cronjob started"}, "")
}
