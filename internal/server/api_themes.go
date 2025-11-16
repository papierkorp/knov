package server

import (
	"net/http"

	"knov/internal/logging"
	"knov/internal/server/render"
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
// @Router /api/themes/setTheme [post]
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