package server

import (
	"fmt"
	"net/http"
	"strings"

	"knov/internal/configmanager"
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
