package server

import (
	"fmt"
	"net/http"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/server/render"
	"knov/internal/thememanager"

	"github.com/go-chi/chi/v5"
)

// @Summary Get themes
// @Description Get current theme and available themes
// @Tags themes
// @Produce json,html
// @Success 200 {string} string "{"current":"themename","available":["theme1","theme2"]}"
// @Router /api/themes [get]
func handleAPIGetThemes(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	currentTheme := tm.GetCurrentTheme()
	availableThemes := tm.GetAvailableThemes()

	response := struct {
		Current   thememanager.Theme   `json:"current"`
		Available []thememanager.Theme `json:"available"`
	}{
		Current:   currentTheme,
		Available: availableThemes,
	}

	html := render.RenderThemeOptions(availableThemes, currentTheme)
	writeResponse(w, r, response, html)
}

// @Summary Set theme
// @Description Set new theme via form parameter
// @Tags themes
// @Accept application/x-www-form-urlencoded
// @Param theme formData string true "Theme name to set"
// @Produce json,html
// @Success 303 "Redirect to settings page"
// @Router /api/themes/ [post]
func handleAPISetTheme(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	themeName := r.FormValue("theme")

	tm := thememanager.GetThemeManager()
	availableThemes := tm.GetAvailableThemes()

	for _, theme := range availableThemes {
		if theme.Name == themeName {
			err := tm.SetCurrentTheme(theme)
			if err != nil {
				logging.LogError("cannot set theme via api: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			logging.LogInfo("theme switched to: %s", themeName)
			w.Header().Set("HX-Refresh", "true")
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// theme not found
	logging.LogError("theme not found: %s", themeName)
	w.WriteHeader(http.StatusBadRequest)
}

// @Summary Get theme settings
// @Description Get all settings for a specific theme
// @Tags themes
// @Param themeName path string true "Theme name"
// @Produce json,html
// @Success 200 {object} object "Theme settings object"
// @Router /api/themes/{themeName}/settings [get]
func handleAPIGetThemeSettings(w http.ResponseWriter, r *http.Request) {
	themeName := chi.URLParam(r, "themeName")

	if themeName == "" {
		http.Error(w, "theme name is required", http.StatusBadRequest)
		return
	}

	settings := configmanager.GetCurrentThemeSettings()
	html := render.RenderThemeSettings(settings, themeName)
	writeResponse(w, r, settings, html)
}

// @Summary Update theme setting
// @Description Update a specific setting for a theme
// @Tags themes
// @Accept application/x-www-form-urlencoded
// @Param themeName path string true "Theme name"
// @Param settingKey path string true "Setting key to update"
// @Param value formData string true "Setting value"
// @Produce json,html
// @Success 200 "Setting updated successfully"
// @Router /api/themes/{themeName}/settings/{settingKey} [put]
func handleAPISetThemeSetting(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// Extract parameters from URL using chi
	themeName := chi.URLParam(r, "themeName")
	settingKey := chi.URLParam(r, "settingKey")
	value := r.FormValue("value")

	if themeName == "" || settingKey == "" {
		http.Error(w, "theme name and setting key are required", http.StatusBadRequest)
		return
	}

	// convert value based on expected type
	var settingValue interface{}
	switch value {
	case "true":
		settingValue = true
	case "false":
		settingValue = false
	default:
		settingValue = value
	}

	configmanager.SetThemeSetting(themeName, settingKey, settingValue)
	logging.LogDebug("theme setting updated: %s.%s = %v", themeName, settingKey, settingValue)

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// @Summary Get theme settings form
// @Description Get all theme settings as HTML form elements
// @Tags themes
// @Produce html
// @Success 200 {string} string "HTML form elements"
// @Router /api/themes/settings [get]
func handleAPIGetThemeSettingsForm(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	schema := tm.GetCurrentThemeSettingsSchema()
	currentValues := configmanager.GetCurrentThemeSettings()

	html := render.RenderThemeSettingsForm(schema, currentValues, "help-text")

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// @Summary Update theme setting
// @Description Update a specific setting for the current theme
// @Tags themes
// @Accept application/x-www-form-urlencoded
// @Param key formData string true "Setting key"
// @Param value formData string true "Setting value"
// @Produce json,html
// @Success 200 "Setting updated successfully"
// @Router /api/themes/settings [post]
func handleAPIUpdateThemeSetting(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	key := r.FormValue("key")
	value := r.FormValue("value")

	if key == "" {
		http.Error(w, "key parameter is required", http.StatusBadRequest)
		return
	}

	currentTheme := configmanager.GetUserSettings().Theme
	tm := thememanager.GetThemeManager()
	schema := tm.GetCurrentThemeSettingsSchema()

	// get setting definition to determine type
	setting, exists := schema[key]
	if !exists {
		http.Error(w, "unknown setting key", http.StatusBadRequest)
		return
	}

	// convert value based on expected type
	var settingValue interface{}
	switch setting.Type {
	case "boolean":
		settingValue = value == "true"
	case "number":
		var num float64
		fmt.Sscanf(value, "%f", &num)
		settingValue = num
	default:
		settingValue = value
	}

	configmanager.SetThemeSetting(currentTheme, key, settingValue)
	logging.LogDebug("theme setting updated: %s = %v", key, settingValue)

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
