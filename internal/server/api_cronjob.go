package server

import (
	"net/http"

	"knov/internal/cronjob"
	"knov/internal/server/render"
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

	data := map[string]string{"status": "ok", "message": "cronjob executed successfully"}
	html := render.RenderStatusMessage(render.StatusOK, "cronjob executed successfully")
	writeResponse(w, r, data, html)
}
