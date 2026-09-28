package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/server/notify"
	"knov/internal/server/render"
	"knov/internal/system"
	"knov/internal/translation"
)

// @Summary Get current configuration
// @Tags config
// @Produce json,html
// @Router /api/config [get]
func handleAPIGetConfig(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	settings := make(map[string]interface{})
	for _, s := range configmanager.AllSettings() {
		settings[s.Key()] = s.GetValue()
	}
	config := map[string]interface{}{
		"app":      appConfig,
		"settings": settings,
	}
	html := render.RenderConfigDisplay(appConfig)
	writeResponse(w, r, config, html)
}

// @Summary Restart application
// @Description Restarts the application in place (same PID on Linux/macOS, so it keeps working under a supervisor like systemd) and standalone otherwise
// @Tags system
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Success 200 {string} string "restarting"
// @Router /api/system/restart [post]
func handleAPIRestartApp(w http.ResponseWriter, r *http.Request) {
	logging.LogInfo(logging.KeyApp, "application restart requested")

	if err := system.CanRestart(); err != nil {
		logging.LogError(logging.KeyApp, "cannot restart: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to restart"))
		return
	}

	data := "restarting"
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "restarting application..."))
	writeResponse(w, r, data, "")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// give the response time to reach the client before this process is replaced/exited below
	time.Sleep(500 * time.Millisecond)
	if err := system.Restart(); err != nil {
		logging.LogError(logging.KeyApp, "failed to restart: %v", err)
		return
	}
	os.Exit(0) // windows only - the in-place restart above never returns on success
}

// @Summary Get available languages
// @Tags config
// @Produce json,html
// @Router /api/config/languages [get]
func handleAPIGetLanguages(w http.ResponseWriter, r *http.Request) {
	languages := configmanager.GetAvailableLanguages()
	currentLang := configmanager.GetLanguage()

	options := render.GetLanguageOptions()
	html := render.RenderSelectOptions(options, currentLang)
	writeResponse(w, r, languages, html)
}

// @Summary Upload custom favicon
// @Description Uploads a custom favicon (ico, png, or svg) stored in storage/favicon
// @Tags config
// @Accept multipart/form-data
// @Param file formData file true "Favicon file (.ico, .png, or .svg)"
// @Produce html
// @Success 200 {string} string "favicon uploaded"
// @Failure 400 {string} string "invalid file"
// @Failure 500 {string} string "upload failed"
// @Router /api/config/favicon [post]
func handleAPIUploadFavicon(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "no file uploaded"))
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".ico" && ext != ".png" && ext != ".svg" {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "only .ico, .png and .svg files are allowed"))
		return
	}

	faviconDir := filepath.Join(configmanager.GetAppConfig().StoragePath, "favicon")
	if err := os.MkdirAll(faviconDir, 0755); err != nil {
		logging.LogError(logging.KeyApp, "favicon upload: failed to create directory: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"))
		return
	}

	destPath := filepath.Join(faviconDir, "favicon"+ext)
	data, err := io.ReadAll(file)
	if err != nil {
		logging.LogError(logging.KeyApp, "favicon upload: failed to read file: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"))
		return
	}

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		logging.LogError(logging.KeyApp, "favicon upload: failed to write file: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"))
		return
	}

	configmanager.SetCustomFaviconExt(ext)

	logging.LogInfo(logging.KeyApp, "favicon uploaded: %s", destPath)
	w.Header().Set("HX-Trigger", "faviconChanged")
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "favicon uploaded"))
	writeResponse(w, r, nil, "")
}

// @Summary Delete custom favicon
// @Description Removes the custom favicon and reverts to the default
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce html
// @Success 200 {string} string "favicon removed"
// @Failure 500 {string} string "failed to remove"
// @Router /api/config/favicon [delete]
func handleAPIDeleteFavicon(w http.ResponseWriter, r *http.Request) {
	ext := configmanager.GetCustomFaviconExt()
	if ext == "" {
		notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "no custom favicon set"))
		writeResponse(w, r, nil, "")
		return
	}

	destPath := filepath.Join(configmanager.GetAppConfig().StoragePath, "favicon", "favicon"+ext)
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		logging.LogError(logging.KeyApp, "favicon delete: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to remove favicon"))
		return
	}

	configmanager.SetCustomFaviconExt("")

	logging.LogInfo(logging.KeyApp, "custom favicon removed")
	w.Header().Set("HX-Trigger", "faviconChanged")
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "custom favicon removed"))
	writeResponse(w, r, nil, "")
}

// @Summary Export user settings as JSON
// @Description Downloads the current user settings as a JSON file
// @Tags config
// @Produce application/json
// @Router /api/config/export [get]
func handleAPIExportSettings(w http.ResponseWriter, r *http.Request) {
	data, err := configmanager.ExportSettingsJSON()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to export settings: %v", err)
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export settings"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"knov-settings.json\"")
	w.Write(data)
}

// @Summary Import user settings from JSON
// @Description Uploads and applies user settings from a JSON file
// @Tags config
// @Accept multipart/form-data
// @Param file formData file true "Settings JSON file"
// @Produce json,html
// @Router /api/config/import [post]
func handleAPIImportSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file"))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"))
		return
	}

	skipped, err := configmanager.ImportSettingsJSON(data)
	if errors.Is(err, configmanager.ErrSaveSettings) {
		writeAPIError(w, r, http.StatusInternalServerError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save setting"))
		return
	}
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid settings file: %s", err.Error()))
		return
	}

	if len(skipped) > 0 {
		logging.LogInfo(logging.KeyApp, "settings imported, %d setting(s) skipped: %s", len(skipped), strings.Join(skipped, ", "))
		notify.SetFlash(notify.LevelWarning, translation.SprintfForRequest(configmanager.GetLanguage(), "settings imported, %d setting(s) skipped - see logs", len(skipped)))
	} else {
		logging.LogInfo(logging.KeyApp, "settings imported successfully")
		notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "settings imported successfully"))
	}
	w.Header().Set("HX-Refresh", "true")
	writeResponse(w, r, map[string]string{"status": "imported"}, "")
}
