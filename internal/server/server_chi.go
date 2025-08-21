// Package server ..
package server

import (
	"fmt"
	"net/http"
	"path/filepath"

	"knov/internal/configmanager"
	"knov/internal/thememanager"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// StartServerChi ...
func StartServerChi() {
	err := configmanager.InitConfig()

	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Starting Chi HTTP server on http://localhost:1324")
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Get("/", handleHome)
	r.Get("/home", handleHome)
	r.Get("/settings", handleSettings)

	r.Get("/static/css/style.css", handleCSS)
	r.Get("/static/css/custom.css", handleCSS)

	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	err = http.ListenAndServe(":1324", r)
	if err != nil {
		fmt.Printf("Error starting chi server: %v\n", err)
		return
	}

}

func handleCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Expires", "0")

	filename := filepath.Base(r.URL.Path)
	var cssPath string

	switch filename {
	case "style.css":
		themeName := thememanager.GetThemeManager().GetCurrentThemeName()
		cssPath = filepath.Join("themes/", themeName, "templates", "style.css")
	case "custom.css":
		cssPath = "config/custom.css"
	}

	http.ServeFile(w, r, cssPath)
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Home()

	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		fmt.Printf("Error loading theme")
	}

	err = component.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		fmt.Printf("Error rendering template: %v\n", err)
		return
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	theme := r.URL.Query().Get("theme")
	if theme != "" {
		tm := thememanager.GetThemeManager()
		currentTheme := tm.GetCurrentThemeName()
		if currentTheme != theme {
			err := tm.LoadTheme(theme)
			if err == nil {
				tm.SetCurrentTheme(theme)
				newConfig := configmanager.ConfigThemes{
					CurrentTheme: theme,
				}
				configmanager.SetConfigThemes(newConfig)
			}
		}
	}

	component, err := thememanager.GetThemeManager().GetCurrentTheme().Settings()

	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		fmt.Printf("Error loading theme")
	}

	err = component.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		fmt.Printf("Error rendering template: %v\n", err)
		return
	}
}
