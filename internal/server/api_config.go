package server

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/translation"
)

// @Summary Get current configuration
// @Tags config
// @Produce json,html
// @Router /api/config [get]
func handleAPIGetConfig(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	userSettings := configmanager.GetUserSettings()

	config := struct {
		App  configmanager.AppConfig    `json:"app"`
		User configmanager.UserSettings `json:"user"`
	}{
		App:  appConfig,
		User: userSettings,
	}

	html := render.RenderConfigDisplay(userSettings, appConfig)
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

// @Summary Set language
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/config/language [post]
func handleAPISetLanguage(w http.ResponseWriter, r *http.Request) {
	lang := r.FormValue("language")

	logging.LogDebug("language set to: %s", lang)

	if lang != "" {
		configmanager.SetLanguage(lang)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// @Summary Get git repository URL
// @Tags config
// @Produce json,html
// @Success 200 {object} string
// @Router /api/config/repository [get]
func handleAPIGetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	repositoryURL := appConfig.GitRepoURL

	html := render.RenderInputField("url", "repositoryURL", "git-url", repositoryURL, translation.SprintfForRequest(configmanager.GetLanguage(), "https://github.com/user/repo.git"), false)
	writeResponse(w, r, repositoryURL, html)
}

// @Summary Update git repository URL
// @Description updates git repository url in .env file (requires restart)
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param repositoryUrl formData string true "repository url"
// @Produce json,html
// @Success 200 {string} string "saved"
// @Router /api/config/repository [post]
func handleAPISetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	repositoryURL := r.FormValue("repositoryUrl")

	if err := configmanager.UpdateEnvFile("KNOV_GIT_REPO_URL", repositoryURL); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save"), http.StatusInternalServerError)
		return
	}

	data := "saved"
	html := render.RenderStatusMessage(render.StatusOK, translation.SprintfForRequest(configmanager.GetLanguage(), "git url saved. restart required."))
	writeResponse(w, r, data, html)
}

// @Summary Restart application
// @Description Restarts the application (requires process manager like systemd or docker)
// @Tags system
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Success 200 {string} string "restarting"
// @Router /api/system/restart [post]
func handleAPIRestartApp(w http.ResponseWriter, r *http.Request) {
	logging.LogInfo("application restart requested")

	data := "restarting"
	html := render.RenderStatusMessage(render.StatusOK, translation.SprintfForRequest(configmanager.GetLanguage(), "restarting application..."))
	writeResponse(w, r, data, html)

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
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save"), http.StatusInternalServerError)
		return
	}

	data := "saved"
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "data path saved. restart required."))
	writeResponse(w, r, data, html)
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

// @Summary Update media upload size limit
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param maxUploadSizeMB formData int true "Maximum upload size in MB"
// @Produce json,html
// @Router /api/config/media/upload-size [post]
func handleAPIUpdateMediaUploadSize(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	sizeStr := r.FormValue("maxUploadSizeMB")
	if sizeStr == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing upload size"), http.StatusBadRequest)
		return
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 1 || size > 100 {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid upload size"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.MaxUploadSizeMB = size

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated media upload size to %d MB", size)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "upload size updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update allowed MIME types
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param allowedMimeTypes formData string true "Comma-separated list of allowed MIME types"
// @Produce json,html
// @Router /api/config/media/mime-types [post]
func handleAPIUpdateMediaMimeTypes(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	mimeTypesStr := r.FormValue("allowedMimeTypes")
	if mimeTypesStr == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing mime types"), http.StatusBadRequest)
		return
	}

	// parse comma-separated list
	var mimeTypes []string
	for _, mimeType := range strings.Split(mimeTypesStr, ",") {
		trimmed := strings.TrimSpace(mimeType)
		if trimmed != "" {
			mimeTypes = append(mimeTypes, trimmed)
		}
	}

	if len(mimeTypes) == 0 {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "at least one mime type required"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.AllowedMimeTypes = mimeTypes

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated allowed mime types: %v", mimeTypes)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "mime types updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update orphaned media behavior
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param orphanedMediaBehavior formData string true "Orphaned media behavior (keep or manual)"
// @Produce json,html
// @Router /api/config/media/orphaned-behavior [post]
func handleAPIUpdateOrphanedBehavior(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	behavior := r.FormValue("orphanedMediaBehavior")
	if behavior != "keep" && behavior != "manual" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid behavior option"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.OrphanedMediaBehavior = behavior

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated orphaned media behavior to: %s", behavior)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "orphaned media behavior updated"))
	writeResponse(w, r, "saved", html)
}

// @Summary Update orphaned media age
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param orphanedMediaAgeDays formData int true "Number of days before orphaned files can be cleaned up"
// @Produce json,html
// @Router /api/config/media/orphaned-age [post]
func handleAPIUpdateOrphanedAge(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid form data"), http.StatusBadRequest)
		return
	}

	ageStr := r.FormValue("orphanedMediaAgeDays")
	if ageStr == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "missing age value"), http.StatusBadRequest)
		return
	}

	age, err := strconv.Atoi(ageStr)
	if err != nil || age < 1 || age > 365 {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "invalid age value"), http.StatusBadRequest)
		return
	}

	// update settings
	userSettings := configmanager.GetUserSettings()
	userSettings.MediaSettings.OrphanedMediaAgeDays = age

	configmanager.SetUserSettings(userSettings)

	logging.LogInfo("updated orphaned media age to %d days", age)
	html := render.RenderStatusMessage("status-ok", translation.SprintfForRequest(configmanager.GetLanguage(), "orphaned media age updated"))
	writeResponse(w, r, "saved", html)
}
