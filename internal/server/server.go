// Package server ..
package server

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/swaggo/http-swagger/v2"
	"knov/internal/plugins"
	_ "knov/internal/server/api" // swaggo api docs
	"knov/internal/thememanager"
)

// StartServerChi ...
func StartServerChi() {

	// ----------------------------------------------------------------------------------------
	// ----------------------------------- define chi server -----------------------------------
	// ----------------------------------------------------------------------------------------

	fmt.Println("Starting Chi HTTP server on http://localhost:1324")
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// ----------------------------------------------------------------------------------------
	// ------------------------------------ default routes ------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Get("/", handleHome)
	r.Get("/home", handleHome)
	r.Get("/settings", handleSettings)

	// ----------------------------------------------------------------------------------------
	// ------------------------------------- static routes -------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Get("/static/css/style.css", handleCSS)
	r.Get("/static/css/custom.css", handleCSS)
	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs)) // else css files are served as text files

	// ----------------------------------------------------------------------------------------
	// ------------------------------------- plugin routes -------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Route("/plugins", func(r chi.Router) {
		r.HandleFunc("/customCSS", plugins.HandleCustomCSS)
	})

	// ----------------------------------------------------------------------------------------
	// -------------------------------------- api routes --------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Get("/swagger/*", httpSwagger.Handler())
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handleAPIHealth)
		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- THEMES ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/themes", func(r chi.Router) {
			r.Get("/getAllThemes", handleAPIGetThemes)
			r.Post("/setTheme", handleAPISetTheme)

		})
		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- CONFIG ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/config", func(r chi.Router) {
			r.Get("/getConfig", handleAPIGetConfig)
			r.Post("/setConfig", handleAPISetConfig)
			r.Post("/setLanguage", handleAPISetLanguage)
			r.Get("/getRepositoryURL", handleAPIGetGitRepositoryURL)
			r.Post("/setDataPath", handleAPISetGitDataPath)
			r.Post("/setRepositoryURL", handleAPISetGitRepositoryURL)
		})
	})

	// ----------------------------------------------------------------------------------------
	// ----------------------------------- start chi server -----------------------------------
	// ----------------------------------------------------------------------------------------

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
	locale := getBrowserLocale(r)
	log.Println("locale: ", locale)
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

func getBrowserLocale(r *http.Request) string {
	acceptLanguage := r.Header.Get("Accept-Language")
	if acceptLanguage == "" {
		// default
		return "en"
	}

	languages := strings.Split(acceptLanguage, ",")
	if len(languages) > 0 {
		locale := strings.Split(strings.TrimSpace(languages[0]), "-")[0]
		return locale
	}

	return "en"
}
