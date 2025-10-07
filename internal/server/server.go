// Package server ..
package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	_ "knov/internal/server/api" // swaggo api docs
	"knov/internal/thememanager"
	"knov/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
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
	r.Get("/admin", handleAdmin)
	r.Get("/playground", handlePlayground)
	r.Get("/latest-changes", handleLatestChanges)
	r.Get("/history", handleHistory)
	r.Get("/overview", handleOverview)
	r.Get("/search", handleSearchPage)
	r.Get("/files/*", handleFileContent)
	r.Get("/dashboard", handleDashboardView)
	r.Get("/dashboard/{id}", handleDashboardView)
	r.Get("/dashboard/new", handleDashboardNew)
	r.Get("/dashboard/edit/{id}", handleDashboardEdit)
	r.Get("/browse/{metadata}/{value}", handleBrowseFiles)

	// ----------------------------------------------------------------------------------------
	// ------------------------------------- static routes -------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Get("/static/css/*", handleCSS)

	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs)) // else css files are served as text files

	// ----------------------------------------------------------------------------------------
	// -------------------------------------- api routes --------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Get("/swagger/*", httpSwagger.Handler())
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handleAPIHealth)
		r.Get("/search", handleAPISearch)

		// ----------------------------------------------------------------------------------------
		// ------------------------------------ system routes ------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/system", func(r chi.Router) {
			r.Post("/restart", handleAPIRestartApp)
		})

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
			r.Post("/customCSS", handleCustomCSS)
			r.Post("/setDataPath", handleAPISetDataPath)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- FILES ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/files", func(r chi.Router) {
			r.Get("/list", handleAPIGetAllFiles)
			r.Get("/content/*", handleAPIGetFileContent)
			r.Post("/filter", handleAPIFilterFiles)
			r.Get("/header", handleAPIGetFileHeader)
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

			r.Get("/tags", handleAPIGetAllTags)
			r.Get("/collections", handleAPIGetAllCollections)
			r.Get("/folders", handleAPIGetAllFolders)
			r.Get("/file/tags", handleAPIGetFileMetadataTags)
			r.Get("/file/folders", handleAPIGetFileMetadataFolders)
			r.Get("/file/collection", handleAPIGetFileMetadataCollection)
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
		// --------------------------------------- DASHBOARDS -------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/dashboards", func(r chi.Router) {
			r.Get("/", handleAPIGetDashboards)
			r.Post("/", handleAPICreateDashboard)
			r.Get("/{id}", handleAPIGetDashboard)
			r.Patch("/{id}", handleAPIUpdateDashboard)
			r.Delete("/{id}", handleAPIDeleteDashboard)
			r.Post("/widget/{id}", handleAPIRenderWidget)
			r.Post("/{id}/rename", handleAPIRenameDashboard)
		})

		// ----------------------------------------------------------------------------------------
		// --------------------------------------- TESTDATA ---------------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/testdata", func(r chi.Router) {
			r.Post("/setup", handleAPISetupTestData)
			r.Post("/clean", handleAPICleanTestData)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------- components routes ----------------------------------
		// ----------------------------------------------------------------------------------------

		r.Route("/components", func(r chi.Router) {
			r.Get("/table", handleAPIGetTable)
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

	switch cssFile {
	case "custom.css":
		customCSS := configmanager.GetUserSettings().CustomCSS
		w.Write([]byte(customCSS))
		return
	case "style.css":
		themeName := thememanager.GetThemeManager().GetCurrentThemeName()
		cssPath := filepath.Join("themes", themeName, "templates", "style.css")
		http.ServeFile(w, r, cssPath)
	default:
		themeName := thememanager.GetThemeManager().GetCurrentThemeName()
		cssPath := filepath.Join("themes", themeName, "templates", cssFile)
		http.ServeFile(w, r, cssPath)
	}
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Home()

	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		fmt.Printf("error loading theme")
	}

	err = component.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		fmt.Printf("error rendering template: %v\n", err)
		return
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Settings()

	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		fmt.Printf("error loading theme")
	}

	err = component.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		fmt.Printf("error rendering template: %v\n", err)
		return
	}
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Admin()

	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		fmt.Printf("error loading theme")
	}

	err = component.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		fmt.Printf("error rendering template: %v\n", err)
		return
	}
}

func handlePlayground(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Playground()

	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		fmt.Printf("error loading theme")
	}

	err = component.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		fmt.Printf("error rendering template: %v\n", err)
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
		fmt.Printf("error rendering template: %v\n", err)
		return
	}
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().History()

	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		fmt.Printf("error loading theme")
	}

	err = component.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		fmt.Printf("error rendering template: %v\n", err)
		return
	}
}

func handleOverview(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Overview()

	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		fmt.Printf("error loading theme")
	}

	err = component.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		fmt.Printf("error rendering template: %v\n", err)
		return
	}
}

func handleSearchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	component, err := thememanager.GetThemeManager().GetCurrentTheme().Search(query)
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

func handleFileContent(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/")
	fullPath := utils.ToFullPath(filePath)
	ext := strings.ToLower(filepath.Ext(fullPath))

	if ext == ".pdf" {
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeFile(w, r, fullPath)
		return
	}

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

	availableViews := tm.GetAvailableFileViews()
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

// @Summary Browse files by metadata
// @Description Browse and filter files by specific metadata type and value
// @Tags files
// @Param metadata path string true "Metadata type" Enums(tags, collection, folders, type, status, priority)
// @Param value path string true "Metadata value to filter by"
// @Produce text/html
// @Success 200 {string} string "rendered browse page"
// @Failure 400 {string} string "missing metadata type or value"
// @Failure 500 {string} string "failed to render page"
// @Router /browse/{metadata}/{value} [get]
func handleBrowseFiles(w http.ResponseWriter, r *http.Request) {
	metadataType := chi.URLParam(r, "metadata")
	value := chi.URLParam(r, "value")

	if metadataType == "" || value == "" {
		http.Error(w, "missing metadata type or value", http.StatusBadRequest)
		return
	}

	query := fmt.Sprintf("%s:%s", metadataType, value)
	component, err := thememanager.GetThemeManager().GetCurrentTheme().BrowseFiles(metadataType, value, query)
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

func handleDashboardNew(w http.ResponseWriter, r *http.Request) {
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Dashboard("", "new")
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handleDashboardEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Dashboard(id, "edit")
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handleDashboardView(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = "home"
	}
	component, err := thememanager.GetThemeManager().GetCurrentTheme().Dashboard(id, "view")
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}
