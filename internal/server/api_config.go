package server

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
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

// @Summary Set git repository URL
// @Tags config
// @Accept application/x-www-form-urlencoded
// @Produce json,html
// @Router /api/config/setRepositoryURL [post]
func handleAPISetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	repositoryURL := r.FormValue("repositoryUrl")

	logging.LogDebug("received repositoryUrl: '%s'", repositoryURL)

	if repositoryURL == "" {
		logging.LogError("empty repositoryUrl")
		http.Error(w, "repositoryUrl cannot be empty", http.StatusBadRequest)
		return
	}

	appConfig := configmanager.GetAppConfig()
	dataDir := appConfig.DataPath

	logging.LogDebug("using datadir: '%s'", dataDir)

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		logging.LogError("data directory doesn't exist: %s", dataDir)
		http.Error(w, fmt.Sprintf("data directory doesn't exist: %s", dataDir), http.StatusInternalServerError)
		return
	}

	logging.LogDebug("attempting to set git remote URL...")

	cmd := exec.Command("git", "remote", "set-url", "origin", repositoryURL)
	cmd.Dir = dataDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		logging.LogError("set-url failed with error: %v, output: %s", err, string(output))
		logging.LogDebug("trying to add remote instead...")

		cmd = exec.Command("git", "remote", "add", "origin", repositoryURL)
		cmd.Dir = dataDir
		output, err = cmd.CombinedOutput()

		if err != nil {
			logging.LogError("add remote failed with error: %v - %s", err, string(output))
			http.Error(w, fmt.Sprintf("git command failed: %v - %s", err, string(output)), http.StatusInternalServerError)
			return
		}
	}

	logging.LogInfo("git remote set successfully")

	data := "saved"
	html := `<span class="status-ok">repository URL saved</span>`
	writeResponse(w, r, data, html)
}
