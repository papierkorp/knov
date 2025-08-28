// Package server ..
package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"strings"

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
	config := configmanager.GetConfig()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// @Summary Set configuration
// @Tags config
// @Router /api/config/setConfig [post]
func handleAPISetConfig(w http.ResponseWriter, r *http.Request) {
	var config configmanager.ConfigManager

	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	configmanager.SetConfig(config)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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
	tm := thememanager.GetThemeManager()

	response := struct {
		Current   string   `json:"current"`
		Available []string `json:"available"`
	}{
		Current:   tm.GetCurrentThemeName(),
		Available: tm.GetAvailableThemes(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// @Summary Set theme
// @Description Set new theme via form parameter
// @Tags themes
// @Accept x-www-form-urlencoded
// @Param theme formData string true "Theme name to set"
// @Success 303 "Redirect to settings page"
// @Router /api/themes/setTheme [post]
func handleAPISetTheme(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	theme := r.FormValue("theme")

	if theme != "" {
		tm := thememanager.GetThemeManager()
		err := tm.LoadTheme(theme)
		if err == nil {
			tm.SetCurrentTheme(theme)
			newConfig := configmanager.ConfigThemes{CurrentTheme: theme}
			configmanager.SetConfigThemes(newConfig)
		}
	}

	// http.Redirect(w, r, "/settings", http.StatusSeeOther)
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// ----------------------------------------------------------------------------------------
// ------------------------------------------ git ------------------------------------------
// ----------------------------------------------------------------------------------------

// @Summary Get git repository URL
// @Tags git
// @Produce json
// @Success 200 {object} string
// @Router /api/git/getRepositoryURL [get]
func handleAPIGetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}
	// Get remote URL from git config
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = dataDir
	output, err := cmd.Output()
	var repositoryURL string
	if err != nil {
		log.Printf("error in git config get remote.origin.url command - using config repositoryURL instead")
		repositoryURL = config.RepositoryURL
		if repositoryURL == "" {
			repositoryURL = "local"
		}
	} else {
		repositoryURL = strings.TrimSpace(string(output))
	}
	response := map[string]string{
		"repositoryUrl": repositoryURL,
		"dataPath":      dataDir,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// @Summary Set git repository URL
// @Tags git
// @Accept json
// @Param repositoryUrl body string true "Repository URL"
// @Success 200 {object} map[string]string
// @Router /api/git/setRepositoryURL [post]
func handleAPISetGitRepositoryURL(w http.ResponseWriter, r *http.Request) {
	var repositoryURL string
	if err := json.NewDecoder(r.Body).Decode(&repositoryURL); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if repositoryURL == "" {
		http.Error(w, "repositoryUrl cannot be empty", http.StatusBadRequest)
		return
	}
	config := configmanager.GetConfigGit()
	dataDir := config.DataPath
	if dataDir == "" {
		dataDir = "data"
	}

	cmd := exec.Command("git", "remote", "set-url", "origin", repositoryURL)
	cmd.Dir = dataDir
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("git", "remote", "add", "origin", repositoryURL)
		cmd.Dir = dataDir
		if err := cmd.Run(); err != nil {
			http.Error(w, "failed to set git remote", http.StatusInternalServerError)
			return
		}
	}
	config.RepositoryURL = repositoryURL
	configmanager.SetConfigGit(config)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
