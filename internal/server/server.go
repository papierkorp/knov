// Package server ..
package server

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/logging"
	_ "knov/internal/server/swagger" // swaggo api docs
	"knov/internal/thememanager"
	"knov/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

var staticFiles embed.FS

func SetStaticFiles(files embed.FS) {
	staticFiles = files
}

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
	r.Get("/files/*", handleFileContent)
	r.Get("/dashboard", handleDashboardView)
	r.Get("/dashboard/{id}", handleDashboardView)
	r.Get("/dashboard/new", handleDashboardNew)
	r.Get("/dashboard/edit/{id}", handleDashboardEdit)
	r.Get("/browse/{metadata}/{value}", handleBrowseFiles)

	// ----------------------------------------------------------------------------------------
	// ------------------------------------- static routes -------------------------------------
	// ----------------------------------------------------------------------------------------

	r.Get("/static/*", handleStatic)
	r.Get("/themes/*", handleThemeStatic)

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
			r.Post("/upload", handleAPIUploadTheme)

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
			r.Post("/save/*", handleAPIFileSave)
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

func handleStatic(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/static/")

	fmt.Printf("handleStatic called for: %s\n", filePath)

	// handle special CSS files
	if strings.HasPrefix(filePath, "css/") {
		w.Header().Set("Content-Type", "text/css")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		cssFile := strings.TrimPrefix(filePath, "css/")

		// Handle custom CSS
		if cssFile == "custom.css" {
			customCSS := configmanager.GetUserSettings().CustomCSS
			w.Write([]byte(customCSS))
			return
		}
	}

	// serve from embedded static files - use forward slashes for embed.FS
	fullPath := "static/" + filePath

	// set content type before serving
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	}

	data, err := staticFiles.ReadFile(fullPath)
	if err != nil {
		fmt.Printf("failed to read embedded file %s: %v\n", fullPath, err)
		http.NotFound(w, r)
		return
	}

	w.Write(data)
}

func handleThemeStatic(w http.ResponseWriter, r *http.Request) {
	// URL format: /themes/{themeName}/static/{path}
	filePath := strings.TrimPrefix(r.URL.Path, "/themes/")

	// Extract theme name and file path
	parts := strings.SplitN(filePath, "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	themeName := parts[0]
	assetPath := parts[1]

	fmt.Printf("handleThemeStatic called for theme: %s, path: %s\n", themeName, assetPath)

	tm := thememanager.GetThemeManager()
	theme := tm.GetCurrentTheme()

	// Security check: only serve files from the current theme
	if theme == nil || theme.Name != themeName {
		http.Error(w, "unauthorized theme access", http.StatusForbidden)
		return
	}

	// Set content type
	ext := strings.ToLower(filepath.Ext(assetPath))
	switch ext {
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".woff", ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	case ".ttf":
		w.Header().Set("Content-Type", "font/ttf")
	}

	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Serve from theme directory (all themes are treated the same)
	themesDir := filepath.Join(configmanager.GetConfigPath(), "themes")
	themePath := filepath.Join(themesDir, themeName, assetPath)
	data, err := os.ReadFile(themePath)
	if err != nil {
		fmt.Printf("failed to read theme file %s: %v\n", themePath, err)
		http.NotFound(w, r)
		return
	}
	w.Write(data)
}

// ----------------------------------------------------------------------------------------
// ------------------------------------ default routes ------------------------------------
// ----------------------------------------------------------------------------------------

func handleHome(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := NewTemplateData("Home")

	if err := tm.RenderPage(w, "home.html", data); err != nil {
		logging.LogError("failed to render home page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := NewTemplateData("Settings")

	if err := tm.RenderPage(w, "settings.html", data); err != nil {
		logging.LogError("failed to render settings page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := NewTemplateData("Admin")

	if err := tm.RenderPage(w, "admin.html", data); err != nil {
		logging.LogError("failed to render admin page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

func handlePlayground(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := NewTemplateData("Playground")

	if err := tm.RenderPage(w, "playground.html", data); err != nil {
		logging.LogError("failed to render playground page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

func handleLatestChanges(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := NewTemplateData("Latest Changes")

	if err := tm.RenderPage(w, "latestchanges.html", data); err != nil {
		logging.LogError("failed to render latestchanges page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := NewTemplateData("History")

	if err := tm.RenderPage(w, "history.html", data); err != nil {
		logging.LogError("failed to render history page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

func handleOverview(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := NewTemplateData("Overview")

	if err := tm.RenderPage(w, "overview.html", data); err != nil {
		logging.LogError("failed to render overview page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

func handleSearchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	tm := thememanager.GetThemeManager()
	data := NewTemplateData("Search").SetSearchData(query)

	if err := tm.RenderPage(w, "search.html", data); err != nil {
		logging.LogError("failed to render search page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

func handleBrowseFiles(w http.ResponseWriter, r *http.Request) {
	metadataType := chi.URLParam(r, "metadata")
	value := chi.URLParam(r, "value")

	if metadataType == "" || value == "" {
		http.Error(w, "missing metadata type or value", http.StatusBadRequest)
		return
	}

	tm := thememanager.GetThemeManager()
	data := NewTemplateData("Browse: "+value).SetBrowseFilesData(metadataType, value)

	if err := tm.RenderPage(w, "browsefiles.html", data); err != nil {
		logging.LogError("failed to render browsefiles page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

func handleDashboardNew(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	data := NewTemplateData("New Dashboard")

	if err := tm.RenderPage(w, "dashboard.html", data); err != nil {
		logging.LogError("failed to render dashboard (new) page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

func handleDashboardEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	dash, err := dashboard.Get(id)
	if err != nil {
		http.Error(w, "dashboard not found", http.StatusNotFound)
		return
	}

	tm := thememanager.GetThemeManager()
	data := NewTemplateData("Edit Dashboard").SetDashboardData(dash)

	if err := tm.RenderPage(w, "dashboard.html", data); err != nil {
		logging.LogError("failed to render dashboard (edit) page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
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
	data := NewTemplateData(dash.Name).SetDashboardData(dash)

	if err := tm.RenderPage(w, "dashboard.html", data); err != nil {
		logging.LogError("failed to render dashboard (view) page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
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
	fileView := configmanager.GetFileView()

	availableViews := tm.GetAvailableViews("fileview")
	if !slices.Contains(availableViews, fileView) && len(availableViews) > 0 {
		fileView = availableViews[0]
		configmanager.SetFileView(fileView)
	}

	data := NewTemplateData(filepath.Base(filePath)).
		SetFileData(fileContent, filePath).
		SetView(fileView)

	if err := tm.RenderPage(w, "fileview.html", data); err != nil {
		logging.LogError("failed to render fileview page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

func handleFileEdit(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/edit/")

	tm := thememanager.GetThemeManager()
	data := NewTemplateData(filepath.Base(filePath) + " - edit")
	data.FilePath = filePath

	if err := tm.RenderPage(w, "fileedit.html", data); err != nil {
		logging.LogError("failed to render fileedit page: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}
