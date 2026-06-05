// Package server ..
package server

import (
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/server/notify"
	"knov/internal/translation"
	"net/http"
)

// @Summary Invalidate cache
// @Description Removes all cache entries, forcing a rebuild on next access
// @Tags system
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Success 200 {string} string "cache invalidated"
// @Failure 500 {string} string "failed to invalidate cache"
// @Router /api/system/cache [delete]
func handleAPIInvalidateCache(w http.ResponseWriter, r *http.Request) {
	if err := files.CacheInvalidate(); err != nil {
		logging.LogError("failed to invalidate cache: %v", err)
		notify.SetFlash(notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to invalidate cache"))
		http.Error(w, "failed to invalidate cache", http.StatusInternalServerError)
		return
	}

	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "cache invalidated"))
	writeResponse(w, r, map[string]string{"status": "cache invalidated"}, "")
}
