package server

import (
	"net/http"
	"os"
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
