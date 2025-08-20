// Package server ..
package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"knov/internal/thememanager"
)

// StartServerChi ...
func StartServerChi() {
	fmt.Println("Starting Chi HTTP server on http://localhost:1324")
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Get("/", handleHome)
	r.Post("/switch-theme", handleSwitchTheme)

	err := http.ListenAndServe(":1324", r)
	if err != nil {
		fmt.Printf("Error starting chi server: %v\n", err)
		return
	}
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

func handleSwitchTheme(w http.ResponseWriter, r *http.Request) {
	themeName := r.FormValue("theme")
	if themeName == "" {
		http.Error(w, "fheme name is required", http.StatusBadRequest)
		return
	}

	tm := thememanager.GetThemeManager()

	currentTheme := thememanager.GetThemeManager().GetCurrentThemeName()
	if currentTheme == themeName {
		fmt.Printf("theme %s already loaded\n", themeName)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	err := tm.LoadTheme(themeName)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load theme: %v", err), http.StatusInternalServerError)
		return
	}

	err = tm.SetCurrentTheme(themeName)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to switch theme: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
