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
var themeManagerFiles embed.FS

func SetStaticFiles(files embed.FS) {
	staticFiles = files
}

// SetThemeManagerFiles sets the embedded thememanager files
func SetThemeManagerFiles(files embed.FS) {
	themeManagerFiles = files
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

	// Handle CSS files from themes
	if strings.HasPrefix(filePath, "css/") {
		w.Header().Set("Content-Type", "text/css")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		cssFile := strings.TrimPrefix(filePath, "css/")

		switch cssFile {
		case "custom.css":
			// User custom CSS
			customCSS := configmanager.GetUserSettings().CustomCSS
			w.Write([]byte(customCSS))
			return
		case "style.css":
			// Theme-specific CSS
			tm := thememanager.GetThemeManager()
			theme := tm.GetCurrentTheme()
			if theme != nil {
				cssPath := filepath.Join(theme.Path, "style.css")
				if data, err := os.ReadFile(cssPath); err == nil {
					w.Write(data)
					return
				}
			}
			// No fallback needed - themes must provide style.css
			http.NotFound(w, r)
			return
		default:
			// Other CSS files from theme
			tm := thememanager.GetThemeManager()
			theme := tm.GetCurrentTheme()
			if theme != nil {
				cssPath := filepath.Join(theme.Path, cssFile)
				if data, err := os.ReadFile(cssPath); err == nil {
					w.Write(data)
					return
				}
			}
			http.NotFound(w, r)
			return
		}
	}

	// Regular static file serving for non-CSS files
	fullPath := "static/" + filePath
	data, err := staticFiles.ReadFile(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set content type
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	}

	w.Write(data)
}

// ----------------------------------------------------------------------------------------
// ------------------------------------ default routes ------------------------------------
// ----------------------------------------------------------------------------------------

func handleHome(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()

	content := thememanager.NewHomeContent(
		"home",
		"welcome to your knowledge management system",
		[]thememanager.QuickAction{
			{Name: "browse files", URL: "/overview", Icon: "Ã°Å¸â€œÂ"},
			{Name: "search", URL: "/search", Icon: "Ã°Å¸â€Â"},
			{Name: "dashboard", URL: "/dashboard", Icon: "Ã°Å¸â€œÅ "},
		},
	)

	if err := tm.Render(w, "home.gotmpl", "default", content); err != nil {
		logging.LogError("template render error: %v", err)
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()

	// Get available views for all view types
	availableViews := make(map[string][]string)
	viewTypes := []string{"file", "home", "search", "overview", "dashboard", "settings", "admin", "playground", "history", "latestchanges", "browsefiles"}
	for _, viewType := range viewTypes {
		availableViews[viewType] = tm.GetAvailableViews(viewType)
	}

	content := thememanager.NewSettingsContent(
		"settings",
		tm.GetCurrentThemeName(),
		configmanager.GetLanguage(),
		availableViews,
	)

	if err := tm.Render(w, "settings.gotmpl", "default", content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()

	systemInfo := thememanager.SystemInfo{
		Version:    "1.0.0", // get from config
		ThemeCount: len(tm.GetThemeNames()),
		FileCount:  0, // get from file manager
	}

	content := thememanager.NewAdminContent("admin panel", systemInfo, tm.GetThemeNames())

	if err := tm.Render(w, "admin.gotmpl", "default", content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
}

func handlePlayground(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()

	content := thememanager.NewPlaygroundContent("playground")

	if err := tm.Render(w, "playground.gotmpl", "default", content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
}

func handleLatestChanges(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()

	// Changes would be populated by git integration
	var changes []thememanager.FileChange
	content := thememanager.NewLatestChangesContent("latest changes", changes)

	if err := tm.Render(w, "latestchanges.gotmpl", "default", content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()

	// Commits would be populated by git integration
	var commits []thememanager.CommitInfo
	content := thememanager.NewHistoryContent("file history", commits)

	if err := tm.Render(w, "history.gotmpl", "default", content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
}

func handleOverview(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()

	// Get configured view
	viewName := configmanager.GetFileView()
	availableViews := tm.GetAvailableViews("overview")
	if !slices.Contains(availableViews, viewName) && len(availableViews) > 0 {
		viewName = availableViews[0]
	}

	content := thememanager.NewOverviewContent("file overview", 0, viewName) // 0 = get from file manager

	if err := tm.Render(w, "overview.gotmpl", viewName, content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
}

func handleSearchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	tm := thememanager.GetThemeManager()

	// Results would be populated by search implementation
	var results []thememanager.SearchResult
	content := thememanager.NewSearchContent("search", query, results)

	if err := tm.Render(w, "search.gotmpl", "default", content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
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

	// Files would be populated by file filter
	var files []thememanager.FileInfo
	content := thememanager.NewBrowseFilesContent("browse files", metadataType, value, files)

	if err := tm.Render(w, "browsefiles.gotmpl", "default", content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
}

func handleDashboardNew(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()

	content := thememanager.NewDashboardContent("new dashboard", "", "new", "")

	if err := tm.Render(w, "dashboard.gotmpl", "default", content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
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

	// Convert dashboard to string representation for now
	dashStr := ""
	if dash != nil {
		dashStr = "dashboard_data" // placeholder - in real implementation would serialize dashboard
	}

	content := thememanager.NewDashboardContent("edit dashboard", id, "edit", dashStr)

	if err := tm.Render(w, "dashboard.gotmpl", "default", content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
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

	// Convert dashboard to string representation for now
	dashStr := ""
	if dash != nil {
		dashStr = "dashboard_data" // placeholder - in real implementation would serialize dashboard
	}

	content := thememanager.NewDashboardContent("dashboard", id, "view", dashStr)

	if err := tm.Render(w, "dashboard.gotmpl", "default", content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
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

	// Get configured view
	fileView := configmanager.GetFileView()
	availableViews := tm.GetAvailableViews("file")
	if !slices.Contains(availableViews, fileView) && len(availableViews) > 0 {
		fileView = availableViews[0]
		configmanager.SetFileView(fileView)
	}

	// Convert fileContent to string for the new system
	fileContentStr := ""
	if fileContent != nil {
		fileContentStr = fileContent.HTML // Use HTML representation
	}

	content := thememanager.NewFileViewContent(filepath.Base(filePath), filePath, fileContentStr, fileView)

	if err := tm.Render(w, "fileview.gotmpl", fileView, content); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
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

	editContent := thememanager.NewFileEditContent("edit file", filePath, content)

	if err := tm.Render(w, "fileedit.gotmpl", "default", editContent); err != nil {
		http.Error(w, "failed to load theme", http.StatusInternalServerError)
		return
	}
}
