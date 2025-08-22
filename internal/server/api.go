// Package server ..
package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/swaggo/http-swagger/v2"
	_ "knov/docs" // swaggo api docs
	// "github.com/go-chi/chi/v5/middleware"
	"knov/internal/configmanager"
	"knov/internal/thememanager"
)

func apiRoutes(r chi.Router) {
	r.Get("/swagger/*", httpSwagger.Handler())
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handleAPIHealth)
		r.Get("/themes", handleAPIThemes)
		r.Post("/themes", handleAPIThemes)
	})
}

// @Summary Health check
// @Router /api/health [get]
func handleAPIHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// @Summary Get and set themes
// @Description Get current theme and available themes, or set new theme via query parameter
// @Tags themes
// @Produce json
// @Param theme query string false "Theme name to set (POST only)"
// @Success 200 {string} string "{"current":"themename","available":["theme1","theme2"]}"
// @Router /api/themes [get]
// @Router /api/themes [post]
func handleAPIThemes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	log.Println("DEBUG r.Method: ", r.Method)

	if r.Method == "POST" {
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
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
	themes := thememanager.GetThemeManager().GetAvailableThemes()
	current := thememanager.GetThemeManager().GetCurrentThemeName()
	response := fmt.Sprintf(`{"current":"%s","available":%v}`, current, themes)
	w.Write([]byte(response))
}
