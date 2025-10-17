package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/thememanager"
)

// @Summary Get themes
// @Description Get current theme and available themes
// @Tags themes
// @Produce json,html
// @Success 200 {string} string "{"current":"themename","available":["theme1","theme2"]}"
// @Router /api/themes/getAllThemes [get]
func handleAPIGetThemes(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	currentTheme := tm.GetCurrentThemeName()
	availableThemes := tm.GetAvailableThemes()

	response := struct {
		Current   string   `json:"current"`
		Available []string `json:"available"`
	}{
		Current:   currentTheme,
		Available: availableThemes,
	}

	var html strings.Builder
	for _, theme := range availableThemes {
		selected := ""
		if theme == currentTheme {
			selected = "selected"
		}
		html.WriteString(fmt.Sprintf(`<option value="%s" %s>%s</option>`, theme, selected, theme))
	}

	writeResponse(w, r, response, html.String())
}

// @Summary Set theme
// @Description Set new theme via form parameter
// @Tags themes
// @Accept application/x-www-form-urlencoded
// @Param theme formData string true "Theme name to set"
// @Produce json,html
// @Success 303 "Redirect to settings page"
// @Router /api/themes/setTheme [post]
func handleAPISetTheme(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	theme := r.FormValue("theme")

	if theme != "" {
		tm := thememanager.GetThemeManager()
		err := tm.SetCurrentTheme(theme)
		if err == nil {
			configmanager.SetTheme(theme)
		} else {
			logging.LogError("failed to set theme: %v", err)
			http.Error(w, "failed to set theme", http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// handleAPIUploadTheme uploads a theme .tgz archive
// @Summary Upload theme
// @Description Upload a theme .tgz archive
// @Tags themes
// @Accept multipart/form-data
// @Param file formData file true "Theme .tgz file"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/themes/upload [post]
func handleAPIUploadTheme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// parse multipart form
	err := r.ParseMultipartForm(50 << 20) // 50MB max
	if err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// validate file extension
	if !strings.HasSuffix(header.Filename, ".tgz") && !strings.HasSuffix(header.Filename, ".tar.gz") {
		http.Error(w, "file must be a .tgz or .tar.gz file", http.StatusBadRequest)
		return
	}

	// extract theme name from filename
	themeName := strings.TrimSuffix(strings.TrimSuffix(header.Filename, ".tgz"), ".tar.gz")

	// load theme from archive
	tm := thememanager.GetThemeManager()
	err = tm.LoadThemeFromArchive(themeName, file)
	if err != nil {
		logging.LogError("failed to load theme from archive: %v", err)
		http.Error(w, fmt.Sprintf("invalid theme archive: %v", err), http.StatusBadRequest)
		return
	}

	logging.LogInfo("theme uploaded and loaded successfully: %s", themeName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "theme uploaded successfully",
		"theme":   themeName,
	})
}
