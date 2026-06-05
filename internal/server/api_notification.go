// Package server - notification flash API handler
package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"knov/internal/logging"
	"knov/internal/notificationStorage"
	"knov/internal/server/render"
)

// @Summary Consume pending flash notification
// @Description Returns the oldest pending notification and marks it as displayed.
// Called once on DOMContentLoaded by the notify JS injected into every page.
// Returns 204 when no pending notification exists.
// @Tags notifications
// @Produce json
// @Success 200 {object} notificationStorage.Notification "pending notification"
// @Success 204 "no pending notification"
// @Router /api/notifications/flash [get]
func handleAPIGetNotificationFlash(w http.ResponseWriter, r *http.Request) {
	n, err := notificationStorage.GetPending()
	if err != nil {
		logging.LogError("failed to get pending notification: %v", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if n == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := notificationStorage.ClearPending(n.ID); err != nil {
		logging.LogError("failed to clear pending notification %s: %v", n.ID, err)
	}

	// always JSON — this endpoint is only called by the JS fetch, never by htmx HTML swap
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(n)
}

// @Summary Get recent notifications
// @Description Returns the most recent notifications from the persistent log, newest first.
// @Tags notifications
// @Produce json,html
// @Param limit query int false "Maximum number to return (default 50)"
// @Success 200 {array} notificationStorage.Notification
// @Router /api/notifications [get]
func handleAPIGetNotifications(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if _, err := fmt.Sscanf(l, "%d", &limit); err != nil || limit < 1 {
			limit = 50
		}
	}

	notifications, err := notificationStorage.GetRecent(limit)
	if err != nil {
		logging.LogError("failed to get notifications: %v", err)
		http.Error(w, "failed to get notifications", http.StatusInternalServerError)
		return
	}

	html := render.RenderNotificationList(notifications)
	writeResponse(w, r, notifications, html)
}

// @Summary Clear all notifications
// @Description Removes all notifications from the persistent log.
// @Tags notifications
// @Produce json
// @Success 200 {object} string "cleared"
// @Router /api/notifications [delete]
func handleAPIDeleteNotifications(w http.ResponseWriter, r *http.Request) {
	if err := notificationStorage.Clear(); err != nil {
		logging.LogError("failed to clear notifications: %v", err)
		http.Error(w, "failed to clear notifications", http.StatusInternalServerError)
		return
	}

	writeResponse(w, r, map[string]string{"status": "cleared"}, "")
}

// @Summary Delete a single notification
// @Description Removes one notification and returns the updated list.
// @Tags notifications
// @Param id path string true "Notification ID"
// @Produce json,html
// @Success 200 {array} notificationStorage.Notification
// @Router /api/notifications/{id} [delete]
func handleAPIDeleteNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	if err := notificationStorage.DeleteByID(id); err != nil {
		logging.LogError("failed to delete notification %s: %v", id, err)
		http.Error(w, "failed to delete notification", http.StatusInternalServerError)
		return
	}

	// return updated list so hx-target="#fp-browse-content" refreshes in place
	notifications, err := notificationStorage.GetRecent(50)
	if err != nil {
		logging.LogError("failed to get notifications after delete: %v", err)
		http.Error(w, "failed to get notifications", http.StatusInternalServerError)
		return
	}

	html := render.RenderNotificationList(notifications)
	writeResponse(w, r, notifications, html)
}
