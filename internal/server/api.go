// Package server - Clean API handlers that delegate to business logic
package server

import (
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/thememanager"
)

// ----------------------------------------------------------------------------------------
// ---------------------------------------- health ----------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Health check
// @Tags health
// @Router /api/health [get]
func handleAPIHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// ----------------------------------------------------------------------------------------
// ---------------------------------------- config ----------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get current configuration
// @Tags config
// @Router /api/config/getConfig [get]
func handleAPIGetConfig(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleGetConfig(w, r)
}

// @Summary Set configuration
// @Tags config
// @Router /api/config/setConfig [post]
func handleAPISetConfig(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleSetConfig(w, r)
}

// @Summary Set language
// @Tags config
// @Router /api/config/setLanguage [post]
func handleAPISetLanguage(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleSetLanguage(w, r)
}

// @Summary Get git repository URL
// @Tags config
// @Produce json
// @Success 200 {object} string
// @Router /api/config/getRepositoryURL [get]
func handleAPIGetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleGetRepositoryURL(w, r)
}

// @Summary Set git data path
// @Tags config
// @Router /api/config/setDataPath [post]
func handleAPISetGitDataPath(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleSetDataPath(w, r)
}

// @Summary Set git repository URL
// @Tags config
// @Router /api/config/setRepositoryURL [post]
func handleAPISetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	configmanager.HandleSetRepositoryURL(w, r)
}

// ----------------------------------------------------------------------------------------
// ---------------------------------------- themes ----------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get themes
// @Description Get current theme and available themes
// @Tags themes
// @Produce json
// @Success 200 {string} string "{"current":"themename","available":["theme1","theme2"]}"
// @Router /api/themes/getAllThemes [get]
func handleAPIGetThemes(w http.ResponseWriter, r *http.Request) {
	thememanager.HandleGetThemes(w, r)
}

// @Summary Set theme
// @Description Set new theme via form parameter
// @Tags themes
// @Accept x-www-form-urlencoded
// @Param theme formData string true "Theme name to set"
// @Success 303 "Redirect to settings page"
// @Router /api/themes/setTheme [post]
func handleAPISetTheme(w http.ResponseWriter, r *http.Request) {
	thememanager.HandleSetTheme(w, r)
}
