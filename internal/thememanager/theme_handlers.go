// Package thememanager - API handlers for themes
package thememanager

import (
	"encoding/json"
	"net/http"

	"knov/internal/configmanager"
)

// HandleGetThemes returns available themes and current theme
func HandleGetThemes(w http.ResponseWriter, r *http.Request) {
	tm := GetThemeManager()

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

// HandleSetTheme sets the current theme from form data
func HandleSetTheme(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	theme := r.FormValue("theme")

	if theme != "" {
		tm := GetThemeManager()
		err := tm.LoadTheme(theme)
		if err == nil {
			tm.SetCurrentTheme(theme)
			newConfig := configmanager.ConfigThemes{CurrentTheme: theme}
			configmanager.SetConfigThemes(newConfig)
		}
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
