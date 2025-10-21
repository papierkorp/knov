// Package main ..
package main

import (
	"embed"
	"fmt"
	"net/http"
	"text/template"
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
	Title:   "Hello World Page",
	Heading: "Welcome to Go Templates!",
	Message: "This is loaded from a template file",
	Items:   []string{"Item 1", "Item 2", "Item 3"},
}

//go:embed templates/*
var templateFS embed.FS

var templates *template.Template
var themeManager IThemeManager

func main() {
	// ----------------------------------
	// ---------- thememanager ----------
	// ----------------------------------

	themeManager = NewThemeManager()
	err := themeManager.Initialize()
	if err != nil {
		fmt.Printf("warning: theme manager initialization failed: %v\n", err)
	}

	// ----------------------------------
	// ------------ templates ------------
	// ----------------------------------
	templates, err = template.ParseFS(templateFS, "templates/*")
	if err != nil {
		panic(fmt.Sprintf("error parsing templates: %v", err))
	}

	fmt.Printf("starting chi http server on http://localhost:1325\n")

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// r.Get("/", handleHome)
	r.Get("/1", handleSimpleStringTemplate)
	r.Get("/2", handleTemplateFile)
	r.Get("/3", handleManager)
	r.Get("/4", handleThemes)
	r.Post("/4", handleThemeChange)
	r.Get("/static/*", handleStatic)

	err = http.ListenAndServe(":1325", r)
	if err != nil {
		fmt.Printf("error starting chi server: %v\n", err)
		return
	}
}

func handleStatic(w http.ResponseWriter, r *http.Request) {
	http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))).ServeHTTP(w, r)
}

// ----------------------------------------------------------------------------
// --------------------------------- strings ---------------------------------
// ----------------------------------------------------------------------------

func handleSimpleStringTemplate(w http.ResponseWriter, _ *http.Request) {
	tmplStr := `
<!doctype html>
<html>
  <head>
    <title>{{.Title}}</title>
    <link rel="stylesheet" href="/static/styles.css" />
  </head>

  <body>
    <div class="container">
      <h1>{{.Heading}}</h1>
      <p>{{.Message}}</p>
      <ul>
        {{range .Items}}
        <li>{{.}}</li>
        {{end}}
      </ul>
    </div>
  </body>
</html>`

	tmpl, err := template.New("").Parse(tmplStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	templateData.Message = ("string template")

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, templateData)
}

// ----------------------------------------------------------------------------
// -------------------------------- html files --------------------------------
// ----------------------------------------------------------------------------

func handleTemplateFile(w http.ResponseWriter, _ *http.Request) {
	filename := "example.html"

	templateData.Message = "template file"
	w.Header().Set("Content-Type", "text/html")

	err := templates.ExecuteTemplate(w, filename, templateData)
	// err := templates.Execute(w, templateData)
	if err != nil {
		fmt.Fprintf(w, "template execution error: %v", err)
		return
	}

	fmt.Printf("template executed successfully\n")
}

// ----------------------------------------------------------------------------
// ------------------------------ templatemanager ------------------------------
// ----------------------------------------------------------------------------

func handleManager(w http.ResponseWriter, _ *http.Request) {
	templateData.Message = "template manager file"

	tmplMgr := NewTemplateManager()

	err := tmplMgr.RenderTemplate(w, "base.gotmpl", templateData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type TemplateManager struct {
	templates *template.Template
}

func NewTemplateManager() *TemplateManager {
	tmpl := template.New("")

	tmpl.Funcs(template.FuncMap{
		"formatDate": func(t time.Time) string {
			return t.Format("January 2, 2006")
		},
	})

	tmpl, err := tmpl.ParseFS(templateFS, "templates/*")
	if err != nil {
		panic(err)
	}

	return &TemplateManager{templates: tmpl}
}

func (tm *TemplateManager) RenderTemplate(w http.ResponseWriter, name string, data TemplateData) error {
	w.Header().Set("Content-Type", "text/html")
	return tm.templates.ExecuteTemplate(w, name, data)
}

// ----------------------------------------------------------------------------
// ------------------------------- thememanager -------------------------------
// ----------------------------------------------------------------------------

func handleThemes(w http.ResponseWriter, r *http.Request) {
	requestedTheme := r.URL.Query().Get("theme")
	if requestedTheme != "" {
		if err := themeManager.SetCurrentTheme(requestedTheme); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	currentTheme := themeManager.GetCurrentTheme()
	if currentTheme == nil {
		http.Error(w, "No theme available", http.StatusInternalServerError)
		return
	}

	data := ThemeData{
		TemplateData:    templateData,
		CurrentTheme:    themeManager.GetCurrentThemeName(),
		CurrentMetadata: currentTheme.Metadata,
		AvailableThemes: themeManager.GetAvailableThemes(),
	}

	data.Message = fmt.Sprintf("Using %s theme (v%s by %s)",
		currentTheme.Metadata.Name,
		currentTheme.Metadata.Version,
		currentTheme.Metadata.Author)

	err := themeManager.Render(w, "base.gotmpl", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleThemeChange(w http.ResponseWriter, r *http.Request) {
	selectedTheme := r.FormValue("theme")
	if selectedTheme != "" {
		if err := themeManager.SetCurrentTheme(selectedTheme); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Redirect back to the themes page
	http.Redirect(w, r, "/4", http.StatusSeeOther)
}
