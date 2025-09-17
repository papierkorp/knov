// Package server - Health API handlers
package server

import (
	"net/http"
)

// @Summary Health check
// @Tags health
// @Produce json,html
// @Router /api/health [get]
func handleAPIHealth(w http.ResponseWriter, r *http.Request) {
	data := map[string]string{"status": "ok"}
	html := `<span class="health-ok">OK</span>`
	writeResponse(w, r, data, html)
}
