package server

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/logging"
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

	var html strings.Builder
	html.WriteString("<div class='config'>")
	html.WriteString(fmt.Sprintf("<p>theme: %s</p>", userSettings.Theme))
	html.WriteString(fmt.Sprintf("<p>language: %s</p>", userSettings.Language))
	html.WriteString(fmt.Sprintf("<p>data path: %s</p>", appConfig.DataPath))
	html.WriteString("</div>")

	writeResponse(w, r, config, html.String())
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

	if repositoryURL == "" {
		repositoryURL = "not configured"
	}

	html := fmt.Sprintf(`<span class="repo-url">%s</span>`, repositoryURL)
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
	html := `<span class="status-ok">git url saved. restart required.</span>`
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

	settings := configmanager.GetUserSettings()
	settings.CustomCSS = css
	configmanager.SetUserSettings(settings)

	data := "css saved"
	html := `<span class="status-ok">custom css saved</span>`
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
	html := `<span class="status-ok">restarting application...</span>`
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
	html := `<span class="status-ok">data path saved. restart required.</span>`
	writeResponse(w, r, data, html)
}

// @Summary Set dark mode
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/config/setDarkMode [post]
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
	metadata := tm.GetCurrentThemeMetadata()

	// if metadata == nil || len(metadata.AvailableColorSchemes) == 0 {
	// 	writeResponse(w, r, []string{}, "<option>no schemes available</option>")
	// 	return
	// }

	// currentScheme := configmanager.GetColorScheme()

	var html strings.Builder
	// for _, scheme := range metadata.AvailableColorSchemes {
	// 	selected := ""
	// 	if scheme.Name == currentScheme {
	// 		selected = "selected"
	// 	}
	// 	html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, scheme.Name, selected, scheme.Label))
	// }

	writeResponse(w, r, metadata, html.String())
}

// @Summary Set color scheme
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/config/setColorScheme [post]
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

	var html strings.Builder
	for _, lang := range languages {
		selected := ""
		if lang.Code == currentLang {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, lang.Code, selected, lang.Name))
	}

	writeResponse(w, r, languages, html.String())
}

// @Summary Get dark mode setting
// @Tags config
// @Produce json,html
// @Router /api/config/getDarkMode [get]
func handleAPIGetDarkMode(w http.ResponseWriter, r *http.Request) {
	darkMode := configmanager.GetDarkMode()

	checked := ""
	if darkMode {
		checked = "checked"
	}
	html := fmt.Sprintf(`<input type="checkbox" name="darkMode" %s hx-post="/api/config/setDarkMode" hx-vals='js:{"enabled": event.target.checked}' hx-trigger="change" />`, checked)

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
	editorHTML := `<textarea name="css" rows="20" style="width: 100%; font-family: monospace;">{{CSS_CONTENT}}</textarea>`
	html := configmanager.GetCustomCSSEditor(editorHTML)

	writeResponse(w, r, configmanager.GetCustomCSS(), html)
}
