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

	response := struct {
		Current   string   `json:"current"`
		Available []string `json:"available"`
	}{
		Current:   tm.GetCurrentThemeName(),
		Available: tm.GetAvailableThemes(),
	}

	var html strings.Builder
	html.WriteString(fmt.Sprintf("<p>Current: <strong>%s</strong></p>", response.Current))
	html.WriteString("<ul>")
	for _, theme := range response.Available {
		html.WriteString(fmt.Sprintf(`<li>%s</li>`, theme))
	}
	html.WriteString("</ul>")

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
