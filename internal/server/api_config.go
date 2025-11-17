package server

import (
	"net/http"
	"os"
	"time"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/thememanager"
)

// @Summary Get current configuration
// @Tags config
// @Produce json,html
// @Router /api/config/getConfig [get]
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
// @Router /api/config/getCurrentDataPath [get]
func handleAPIGetCurrentDataPath(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	dataPath := appConfig.DataPath

	html := render.RenderInputField("text", "dataPath", "data-path", dataPath, "/path/to/data", true)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Set language
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/config/setLanguage [post]
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
// @Router /api/config/getRepositoryURL [get]
func handleAPIGetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	appConfig := configmanager.GetAppConfig()
	repositoryURL := appConfig.GitRepoURL

	html := render.RenderInputField("url", "repositoryURL", "git-url", repositoryURL, "https://github.com/user/repo.git", false)
	writeResponse(w, r, repositoryURL, html)
}

// @Summary Update git repository URL
// @Description updates git repository url in .env file (requires restart)
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param repositoryUrl formData string true "repository url"
// @Produce json,html
// @Success 200 {string} string "saved"
// @Router /api/config/setRepositoryURL [post]
func handleAPISetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	repositoryURL := r.FormValue("repositoryUrl")

	if err := configmanager.UpdateEnvFile("KNOV_GIT_REPO_URL", repositoryURL); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}

	data := "saved"
	html := render.RenderStatusMessage(render.StatusOK, "git url saved. restart required.")
	writeResponse(w, r, data, html)
}

// @Summary Save custom CSS for current user
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Param css formData string true "CSS content"
// @Produce json,html
// @Success 200 {string} string "css saved"
// @Router /api/config/customCSS [post]
func handleCustomCSS(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	css := r.FormValue("css")

	configmanager.SetCustomCSS(css)

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
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
	html := render.RenderStatusMessage(render.StatusOK, "restarting application...")
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
// @Router /api/config/setDataPath [post]
func handleAPISetDataPath(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	dataPath := r.FormValue("dataPath")

	if dataPath == "" {
		http.Error(w, "data path cannot be empty", http.StatusBadRequest)
		return
	}

	if err := configmanager.UpdateEnvFile("KNOV_DATA_PATH", dataPath); err != nil {
		logging.LogError("failed to update env file: %v", err)
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}

	data := "saved"
	html := render.RenderStatusMessage("status-ok", "data path saved. restart required.")
	writeResponse(w, r, data, html)
}

// @Summary Set dark mode
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/config/darkmode [post]
func handleAPISetDarkMode(w http.ResponseWriter, r *http.Request) {
	enabled := r.FormValue("enabled") == "true"
	configmanager.SetDarkMode(enabled)
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// @Summary Get available color schemes
// @Tags config
// @Produce json,html
// @Router /api/config/getColorSchemes [get]
func handleAPIGetColorSchemes(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	themeSettings := tm.GetCurrentThemeSettingsSchema()

	colorSchemeSetting, exists := themeSettings["colorScheme"]
	if !exists || len(colorSchemeSetting.Options) == 0 {
		writeResponse(w, r, []string{}, "<option>no color schemes available</option>")
		return
	}

	currentScheme := configmanager.GetColorScheme()

	// Convert theme setting options to htmx select options
	options := make([]render.SelectOption, len(colorSchemeSetting.Options))
	for i, option := range colorSchemeSetting.Options {
		options[i] = render.SelectOption{
			Value: option,
			Label: option,
		}
	}

	html := render.RenderSelectOptions(options, currentScheme)
	writeResponse(w, r, colorSchemeSetting.Options, html)
}

// @Summary Set color scheme
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/config/colorschemes [post]
func handleAPISetColorScheme(w http.ResponseWriter, r *http.Request) {
	scheme := r.FormValue("colorScheme")

	if scheme != "" {
		configmanager.SetColorScheme(scheme)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// @Summary Get available languages
// @Tags config
// @Produce json,html
// @Router /api/config/getLanguages [get]
func handleAPIGetLanguages(w http.ResponseWriter, r *http.Request) {
	languages := configmanager.GetAvailableLanguages()
	currentLang := configmanager.GetLanguage()

	options := render.GetLanguageOptions()
	html := render.RenderSelectOptions(options, currentLang)
	writeResponse(w, r, languages, html)
}

// @Summary Get dark mode setting
// @Tags config
// @Produce json,html
// @Router /api/config/darkmode [get]
func handleAPIGetDarkMode(w http.ResponseWriter, r *http.Request) {
	darkMode := configmanager.GetDarkMode()
	html := render.RenderCheckbox("darkMode", "/api/config/darkmode", darkMode, `hx-vals='js:{"enabled": event.target.checked}' hx-trigger="change"`)
	writeResponse(w, r, darkMode, html)
}

// @Summary Get dark mode status as boolean
// @Tags config
// @Produce json,html
// @Router /api/config/getDarkModeStatus [get]
func handleAPIGetDarkModeStatus(w http.ResponseWriter, r *http.Request) {
	darkMode := configmanager.GetDarkMode()

	if darkMode {
		w.Write([]byte("true"))
	} else {
		w.Write([]byte("false"))
	}
}

// @Summary Get custom CSS
// @Tags config
// @Produce json,html
// @Router /api/config/getCustomCSS [get]
func handleAPIGetCustomCSS(w http.ResponseWriter, r *http.Request) {
	html := render.RenderCustomCSSTextarea(configmanager.GetCustomCSS())
	writeResponse(w, r, configmanager.GetCustomCSS(), html)
}
