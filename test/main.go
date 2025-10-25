package main

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed themes/builtin
var builtinTheme embed.FS

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

	fmt.Printf("starting chi http server on http://localhost:1325\n")
	http.ListenAndServe(":1325", r)
}

func handleCSS(w http.ResponseWriter, r *http.Request) {
	cssFile := strings.TrimPrefix(r.URL.Path, "/")

	// todo: test for css files instead of * wildcard
	if _, err := os.Stat(cssFile); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/css")
	http.ServeFile(w, r, cssFile)
}

func handleBase(w http.ResponseWriter, r *http.Request) {
	err := themeManager.Render(w, "base", "")
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	err := themeManager.Render(w, "settings", "")
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
