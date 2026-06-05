// Package server - notification flash API handler
package server

import (
	"encoding/json"
	"net/http"

	"knov/internal/server/notify"
)

// @Summary Consume flash notification
// @Description Reads and deletes any pending cross-navigation flash notification
// from cache storage, firing it as an HX-Trigger event on the new page.
// Called automatically on DOMContentLoaded by the notify JS injected into every page.
// @Tags notifications
// @Produce json
// @Success 200 "flash notification fired via HX-Trigger"
// @Success 204 "no pending flash"
// @Router /api/notifications/flash [get]
func handleAPIGetNotificationFlash(w http.ResponseWriter, r *http.Request) {
	p := notify.ConsumeFlash()
	if p == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// re-emit as HX-Trigger so the existing JS listener fires the toast
	data, err := json.Marshal(map[string]any{"notify": p})
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("HX-Trigger", string(data))
	writeResponse(w, r, p, "")
}
