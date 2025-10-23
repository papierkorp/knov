package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type TemplateData struct {
	Name    string
	Time    time.Time
	Title   string
	Heading string
	Message string
	Items   []string
}

var templateData = TemplateData{
	Name:    "World",
	Time:    time.Now(),
	Title:   "Theme Demo",
	Heading: "Go Template Themes",
	Message: "Multi-template theme system",
	Items:   []string{"Feature 1", "Feature 2", "Feature 3"},
}

var themeManager *ThemeManager

func main() {
	// Initialize theme manager
	themeManager = NewThemeManager()
	if err := themeManager.LoadThemes("./themes"); err != nil {
		fmt.Printf("Error loading themes: %v\n", err)
		fmt.Println("Make sure you have themes/dark and themes/light directories with all required templates")
		return
	}

	fmt.Printf("Starting server on http://localhost:1325\n")
	fmt.Printf("Available pages: /?page=base, /?page=history, /?page=fileview\n")

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", handlePage)
	r.Post("/", handleThemeChange)
	r.Get("/static/*", handleStatic)

	http.ListenAndServe(":1325", r)
}

func handleStatic(w http.ResponseWriter, r *http.Request) {
	http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))).ServeHTTP(w, r)
}

func handlePage(w http.ResponseWriter, r *http.Request) {
	// Handle theme query parameter
	if theme := r.URL.Query().Get("theme"); theme != "" {
		if err := themeManager.SetTheme(theme); err != nil {
			http.Error(w, fmt.Sprintf("Theme error: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Determine which page to show
	page := r.URL.Query().Get("page")
	var templateName string

	switch page {
	case "history":
		templateName = "history.gotmpl"
		templateData.Message = "Viewing application history"
	case "fileview":
		templateName = "fileview.gotmpl"
		templateData.Message = "Browsing files and directories"
	default:
		templateName = "base.gotmpl"
		templateData.Message = "Welcome to the main page"
	}

	templateData.Time = time.Now()

	// This will now show proper error messages if templates are missing
	if err := themeManager.Render(w, templateName, templateData); err != nil {
		http.Error(w, fmt.Sprintf("Render error: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleThemeChange(w http.ResponseWriter, r *http.Request) {
	if theme := r.FormValue("theme"); theme != "" {
		if err := themeManager.SetTheme(theme); err != nil {
			http.Error(w, fmt.Sprintf("Theme error: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Preserve the current page after theme change
	page := r.FormValue("page")
	if page == "" {
		page = "base"
	}

	http.Redirect(w, r, "/?page="+page, http.StatusSeeOther)
}
