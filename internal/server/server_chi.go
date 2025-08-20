// Package server ..
package server

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

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

	r.Get("/static/css/style.css", handleCSS)
	r.Get("/static/css/custom.css", handleCSS)

	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	err := http.ListenAndServe(":1324", r)
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

	log.Println("cssfilename: ", filename)

	switch filename {
	case "style.css":
		themeName := thememanager.GetThemeManager().GetCurrentThemeName()
		cssPath = filepath.Join("data/themes", themeName, "templates", "style.css")
	case "custom.css":
		cssPath = "data/custom.css"
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

	// http.Redirect(w, r, "/", http.StatusSeeOther)
	redirectURL := fmt.Sprintf("/?v=%d", time.Now().Unix())
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
