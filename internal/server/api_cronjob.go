package server

import (
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/cronjob"
	"knov/internal/server/notify"
	"knov/internal/translation"
)

// @Summary Run cronjob
// @Description Manually triggers the cronjob execution (file processing and search indexing)
// @Tags cronjob
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Success 200 {object} string "{"status":"ok","message":"cronjob executed successfully"}"
// @Failure 500 {object} string "Internal server error"
// @Router /api/cronjob [post]
func handleAPIRunCronjob(w http.ResponseWriter, r *http.Request) {
	cronjob.Run()
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "cronjob executed successfully"))
	writeResponse(w, r, map[string]string{"status": "ok", "message": "cronjob executed successfully"}, "")
}
