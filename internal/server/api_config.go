package server

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/server/notify"
	"knov/internal/server/render"
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

// @Summary Get current data path as input field
// @Tags config
// @Produce html
// @Router /api/config/datapath [get]
func handleAPIGetCurrentDataPath(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	dataPath := appConfig.DataPath

	html := render.RenderInputField("text", "dataPath", "data-path", dataPath, translation.SprintfForRequest(configmanager.GetLanguage(), "/path/to/data"), true)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Get git remote URL
// @Tags config
// @Produce json,html
// @Success 200 {object} string
// @Router /api/config/repository [get]
func handleAPIGetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	repositoryURL := appConfig.GitRemote

	html := render.RenderInputField("text", "repositoryURL", "git-url", repositoryURL, translation.SprintfForRequest(configmanager.GetLanguage(), "https://github.com/user/repo.git or git@github.com:user/repo.git"), false)
	writeResponse(w, r, repositoryURL, html)
}

// @Summary Update git remote URL
// @Description updates git remote url in .env file
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param repositoryURL formData string true "remote repository url"
// @Produce json,html
// @Success 200 {string} string "saved"
// @Router /api/config/repository [post]
func handleAPISetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	repositoryURL := r.FormValue("repositoryURL")

	if err := configmanager.UpdateEnvFile("KNOV_GIT_REMOTE", repositoryURL); err != nil {
		logging.LogError(logging.KeyApp, "failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save"), http.StatusInternalServerError)
		return
	}

	if err := git.EnsureRemote(); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to configure git remote: %v", err)
	}

	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "git remote saved"))
	writeResponse(w, r, "saved", "")
}

// @Summary Restart application
// @Description Restarts the application (requires process manager like systemd or docker)
// @Tags system
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Success 200 {string} string "restarting"
// @Router /api/system/restart [post]
func handleAPIRestartApp(w http.ResponseWriter, r *http.Request) {
	logging.LogInfo(logging.KeyApp, "application restart requested")

	data := "restarting"
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "restarting application..."))
	writeResponse(w, r, data, "")

	// give response time to send
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

// @Summary Update data path
// @Description updates data path in .env file (requires restart)
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param dataPath formData string true "data path"
// @Produce json,html
// @Success 200 {string} string "saved"
// @Router /api/config/datapath [post]
func handleAPISetDataPath(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	dataPath := r.FormValue("dataPath")

	if dataPath == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "data path cannot be empty"), http.StatusBadRequest)
		return
	}

	if err := configmanager.UpdateEnvFile("KNOV_DATA_PATH", dataPath); err != nil {
		logging.LogError(logging.KeyApp, "failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save"), http.StatusInternalServerError)
		return
	}

	data := "saved"
	notify.SetHeader(w, notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "data path saved. restart required."))
	writeResponse(w, r, data, "")
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
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"))
		writeResponse(w, r, nil, "")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "no file uploaded"))
		writeResponse(w, r, nil, "")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".ico" && ext != ".png" && ext != ".svg" {
		w.WriteHeader(http.StatusBadRequest)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "only .ico, .png and .svg files are allowed"))
		writeResponse(w, r, nil, "")
		return
	}

	faviconDir := filepath.Join(configmanager.GetAppConfig().StoragePath, "favicon")
	if err := os.MkdirAll(faviconDir, 0755); err != nil {
		logging.LogError(logging.KeyApp, "favicon upload: failed to create directory: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to create directory"))
		writeResponse(w, r, nil, "")
		return
	}

	destPath := filepath.Join(faviconDir, "favicon"+ext)
	data, err := io.ReadAll(file)
	if err != nil {
		logging.LogError(logging.KeyApp, "favicon upload: failed to read file: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"))
		writeResponse(w, r, nil, "")
		return
	}

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		logging.LogError(logging.KeyApp, "favicon upload: failed to write file: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save file"))
		writeResponse(w, r, nil, "")
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
		w.WriteHeader(http.StatusInternalServerError)
		notify.SetHeader(w, notify.LevelError, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to remove favicon"))
		writeResponse(w, r, nil, "")
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
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to export settings"), http.StatusInternalServerError)
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
// @Produce html
// @Router /api/config/import [post]
func handleAPIImportSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing file"), http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to read file"), http.StatusBadRequest)
		return
	}

	if err := configmanager.ImportSettingsJSON(data); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid settings file"), http.StatusBadRequest)
		return
	}

	logging.LogInfo(logging.KeyApp, "settings imported successfully")
	notify.SetFlash(notify.LevelSuccess, translation.SprintfForRequest(configmanager.GetLanguage(), "settings imported successfully"))
	w.Header().Set("HX-Refresh", "true")
	writeResponse(w, r, map[string]string{"status": "imported"}, "")
}
