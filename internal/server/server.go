// Package server ..
package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/plugins"
	_ "knov/internal/server/api" // swaggo api docs
	"knov/internal/thememanager"
	"knov/internal/utils"
)

// StartServerChi ...
func StartServerChi() {

	// ----------------------------------------------------------------------------------------
	// ----------------------------------- define chi server -----------------------------------
	// ----------------------------------------------------------------------------------------
	appConfig := configmanager.GetAppConfig()
	port := appConfig.ServerPort

	fmt.Printf("starting chi http server on http://localhost:%s\n", port)
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// ----------------------------------------------------------------------------------------
	// ------------------------------------ default routes ------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Get("/", handleHome)
	r.Get("/home", handleHome)
	r.Get("/settings", handleSettings)
	r.Get("/playground", handlePlayground)
	r.Get("/latest-changes", handleLatestChanges)
	r.Get("/history", handleHistory)
	r.Get("/overview", handleOverview)

	r.Get("/files/*", handleFileContent)

	// ----------------------------------------------------------------------------------------
	// ------------------------------------- static routes -------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Get("/static/css/*", handleCSS)

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
		r.Get("/search", handleAPISearch)
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
			r.Post("/setLanguage", handleAPISetLanguage)
			r.Get("/getRepositoryURL", handleAPIGetGitRepositoryURL)
			r.Post("/setRepositoryURL", handleAPISetGitRepositoryURL)
			r.Get("/getAvailableFileViews", handleAPIGetAvailableFileViews)
			r.Post("/setFileView", handleAPISetFileView)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- FILES ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/files", func(r chi.Router) {
			r.Get("/list", handleAPIGetAllFiles)
			r.Get("/content/*", handleAPIGetFileContent)
			r.Post("/filter", handleAPIFilterFiles)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- METADATA ---------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/metadata", func(r chi.Router) {
			r.Get("/", handleAPIGetMetadata)
			r.Post("/", handleAPISetMetadata)
			r.Post("/rebuild", handleAPIRebuildMetadata)

			r.Get("/collection", handleAPIGetMetadataCollection)
			r.Get("/filetype", handleAPIGetMetadataFileType)
			r.Get("/status", handleAPIGetMetadataStatus)
			r.Get("/priority", handleAPIGetMetadataPriority)
			r.Get("/path", handleAPIGetMetadataPath)
			r.Get("/name", handleAPIGetMetadataName)
			r.Get("/createdat", handleAPIGetMetadataCreatedAt)
			r.Get("/lastedited", handleAPIGetMetadataLastEdited)
			r.Get("/folders", handleAPIGetMetadataFolders)

			r.Post("/collection", handleAPISetMetadataCollection)
			r.Post("/filetype", handleAPISetMetadataFileType)
			r.Post("/status", handleAPISetMetadataStatus)
			r.Post("/priority", handleAPISetMetadataPriority)
			r.Post("/path", handleAPISetMetadataPath)
			r.Post("/name", handleAPISetMetadataName)
			r.Post("/createdat", handleAPISetMetadataCreatedAt)
			r.Post("/lastedited", handleAPISetMetadataLastEdited)
			r.Post("/folders", handleAPISetMetadataFolders)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- LINKS ------------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/links", func(r chi.Router) {
			r.Get("/parents", handleAPIGetParents)
			r.Get("/ancestors", handleAPIGetAncestors)
			r.Get("/kids", handleAPIGetKids)
			r.Get("/used", handleAPIGetUsedLinks)
			r.Get("/linkstohere", handleAPIGetLinksToHere)
		})

		// ----------------------------------------------------------------------------------------
		// ------------------------------------ GIT Operations ------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/git", func(r chi.Router) {
			r.Get("/latestchanges", handleAPIGetRecentlyChanged)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- TESTDATA ---------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/testdata", func(r chi.Router) {
			r.Post("/setup", handleAPISetupTestData)
			r.Post("/clean", handleAPICleanTestData)
		})
	})

	// ----------------------------------------------------------------------------------------
	// ----------------------------------- start chi server -----------------------------------
	// ----------------------------------------------------------------------------------------

	err := http.ListenAndServe(":"+port, r)
	if err != nil {
		fmt.Printf("error starting chi server: %v\n", err)
		return
	}
}

func handleCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	cssFile := strings.TrimPrefix(r.URL.Path, "/static/css/")
	var cssPath string

	switch cssFile {
	case "style.css":
		themeName := thememanager.GetThemeManager().GetCurrentThemeName()
		cssPath = filepath.Join("themes", themeName, "templates", "style.css")
	case "custom.css":
		cssPath = "config/custom.css"
	default:
		themeName := thememanager.GetThemeManager().GetCurrentThemeName()
		cssPath = filepath.Join("themes", themeName, "templates", cssFile)
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

func handlePlayground(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Playground()

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

func handleLatestChanges(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().LatestChanges()

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

func handleHistory(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().History()

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

func handleOverview(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Overview()

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

func handleFileContent(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/")
	fullPath := utils.ToFullPath(filePath)

	content, err := files.GetFileContent(fullPath)
	if err != nil {
		http.Error(w, "failed to get file content", http.StatusInternalServerError)
		return
	}

	if r.URL.Query().Get("snippet") == "true" || r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		w.Write(content)
		return
	}

	tm := thememanager.GetThemeManager()
	currentTheme := tm.GetCurrentTheme()
	fileView := configmanager.GetFileView()

	availableViews := currentTheme.GetAvailableFileViews()
	if !slices.Contains(availableViews, fileView) && len(availableViews) > 0 {
		fileView = availableViews[0]
		configmanager.SetFileView(fileView)
	}

	component, err := currentTheme.RenderFileView(fileView, string(content), filePath)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}

	err = component.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		return
	}
}
