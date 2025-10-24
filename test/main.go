package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	InitThemeManager()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", handleBase)
	r.Get("/base", handleBase)
	r.Get("/settings", handleSettings)
	r.Post("/settings", handleThemeChange)
	r.Get("/*", handleCSS)
	r.Get("/themes/"+string(themeManager.currentTheme.Name)+"/*", handleCSS)

	http.ListenAndServe(":1325", r)
}

func handleCSS(w http.ResponseWriter, r *http.Request) {
	// todo: use themespath config
	cssFile := strings.TrimPrefix(r.URL.Path, "/")
	cssPath := filepath.Join("themes", string(themeManager.currentTheme.Name), cssFile)

	// todo: test for css files instead of * wildcard
	if _, err := os.Stat(cssPath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	fmt.Printf("cssFile: %s", cssFile)
	fmt.Printf("cssPath: %s", cssPath)
	w.Header().Set("Content-Type", "text/css")
	// w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	// w.Header().Set("Pragma", "no-cache")
	// w.Header().Set("Expires", "0")
	http.ServeFile(w, r, cssPath)

}

func handleBase(w http.ResponseWriter, r *http.Request) {
	err := themeManager.Render(w, "base")
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	err := themeManager.Render(w, "settings")
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleThemeChange(w http.ResponseWriter, r *http.Request) {
	themeName := r.FormValue("theme")

	for _, theme := range themeManager.themes {
		if string(theme.Name) == themeName {
			themeManager.setCurrentTheme(theme)
			break
		}
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
