package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
		err := tm.LoadTheme(theme)
		if err == nil {
			tm.SetCurrentTheme(theme)
			configmanager.SetTheme(theme)
		}
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// handleAPIUploadTheme uploads a self-contained theme .so file
// @Summary Upload theme
// @Description Upload a self-contained theme .so file
// @Tags themes
// @Accept multipart/form-data
// @Param file formData file true "Theme .so file"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/themes/upload [post]
func handleAPIUploadTheme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10MB max
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
	if !strings.HasSuffix(header.Filename, ".so") {
		http.Error(w, "file must be a .so file", http.StatusBadRequest)
		return
	}

	// extract theme name from filename
	themeName := strings.TrimSuffix(header.Filename, ".so")

	// create themes directory if it doesn't exist
	themesPath := configmanager.GetThemesPath()
	if err := os.MkdirAll(themesPath, 0755); err != nil {
		http.Error(w, "failed to create themes directory", http.StatusInternalServerError)
		return
	}

	// create destination file
	destPath := filepath.Join("themes", header.Filename)
	destFile, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "failed to create destination file", http.StatusInternalServerError)
		return
	}
	defer destFile.Close()

	// copy uploaded file to destination
	_, err = io.Copy(destFile, file)
	if err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}

	// try to load the theme to validate it
	tm := thememanager.GetThemeManager()
	err = tm.LoadTheme(themeName)
	if err != nil {
		// remove invalid file
		os.Remove(destPath)
		http.Error(w, fmt.Sprintf("invalid theme file: %v", err), http.StatusBadRequest)
		return
	}

	logging.LogInfo("theme uploaded successfully: %s", themeName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "theme uploaded successfully",
		"theme":   themeName,
	})
}
