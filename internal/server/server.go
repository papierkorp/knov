// Package server ..
package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
	_ "knov/internal/server/swagger" // swaggo api docs
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
	r.Get("/files/edit/*", handleFileEdit)
	r.Post("/files/save/*", handleAPIFileSave)
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
	r.Get("/static/*", handleStatic)

	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

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
			r.Post("/setDarkMode", handleAPISetDarkMode)
			r.Get("/getColorSchemes", handleAPIGetColorSchemes)
			r.Post("/setColorScheme", handleAPISetColorScheme)
		})

		// ----------------------------------------------------------------------------------------
		// ---------------------------------------- FILES ----------------------------------------
		// ----------------------------------------------------------------------------------------
		r.Route("/files", func(r chi.Router) {
			r.Get("/list", handleAPIGetAllFiles)
			r.Get("/content/*", handleAPIGetFileContent)
			r.Post("/filter", handleAPIFilterFiles)
			r.Get("/header", handleAPIGetFileHeader)
			r.Get("/raw", handleAPIGetRawContent)
			r.Post("/save", handleAPIFileSave)
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
			r.Get("/editor", handleAPIGetEditor)
			r.Get("/markdown-editor", handleAPIGetMarkdownEditor)
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

// ----------------------------------------------------------------------------------------
// ---------------------------------------- helper ----------------------------------------
// ----------------------------------------------------------------------------------------

func getViewName(tm thememanager.IThemeManager, viewType string) string {
	views := tm.GetAvailableViews(viewType)
	if len(views) > 0 {
		return views[0]
	}
	return "default"
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

func handleStatic(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/static/")
	fullPath := filepath.Join("static", filePath)

	// set correct content type based on extension
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	}

	http.ServeFile(w, r, fullPath)
}

// ----------------------------------------------------------------------------------------
// ------------------------------------ default routes ------------------------------------
// ----------------------------------------------------------------------------------------

func handleHome(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "home")

	component, err := tm.GetCurrentTheme().Home(viewName)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "settings")

	component, err := tm.GetCurrentTheme().Settings(viewName)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "admin")

	component, err := tm.GetCurrentTheme().Admin(viewName)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handlePlayground(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "playground")

	component, err := tm.GetCurrentTheme().Playground(viewName)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handleLatestChanges(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "latestchanges")

	component, err := tm.GetCurrentTheme().LatestChanges(viewName)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "history")

	component, err := tm.GetCurrentTheme().History(viewName)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handleOverview(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "overview")

	component, err := tm.GetCurrentTheme().Overview(viewName)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handleSearchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "search")

	component, err := tm.GetCurrentTheme().Search(viewName, query)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handleBrowseFiles(w http.ResponseWriter, r *http.Request) {
	metadataType := chi.URLParam(r, "metadata")
	value := chi.URLParam(r, "value")

	if metadataType == "" || value == "" {
		http.Error(w, "missing metadata type or value", http.StatusBadRequest)
		return
	}

	query := fmt.Sprintf("%s:%s", metadataType, value)
	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "browsefiles")

	component, err := tm.GetCurrentTheme().BrowseFiles(viewName, metadataType, value, query)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handleDashboardNew(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "dashboard")

	component, err := tm.GetCurrentTheme().Dashboard(viewName, "", "new", nil)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}

func handleDashboardEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	dash, err := dashboard.Get(id)
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "dashboard")

	component, err := tm.GetCurrentTheme().Dashboard(viewName, id, "edit", dash)
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

	dash, err := dashboard.Get(id)
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "dashboard")

	component, err := tm.GetCurrentTheme().Dashboard(viewName, id, "view", dash)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
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

	fileContent, err := files.GetFileContent(fullPath)
	if err != nil {
		http.Error(w, "failed to get file content", http.StatusInternalServerError)
		return
	}

	if r.URL.Query().Get("snippet") == "true" || r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fileContent.HTML))
		return
	}

	tm := thememanager.GetThemeManager()
	currentTheme := tm.GetCurrentTheme()
	fileView := configmanager.GetFileView()

	availableViews := tm.GetAvailableViews("file")
	if !slices.Contains(availableViews, fileView) && len(availableViews) > 0 {
		fileView = availableViews[0]
		configmanager.SetFileView(fileView)
	}

	component, err := currentTheme.RenderFileView(fileView, fileContent, filePath)
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

func handleFileEdit(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/edit/")
	fullPath := utils.ToFullPath(filePath)

	content, err := files.GetRawContent(fullPath)
	if err != nil {
		content = ""
	}

	tm := thememanager.GetThemeManager()
	viewName := getViewName(tm, "fileedit")

	component, err := tm.GetCurrentTheme().FileEdit(viewName, content, filePath)
	if err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
	component.Render(r.Context(), w)
}
