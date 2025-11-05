// Package server ..
package server

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
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
	r.Get("/themes/*", handleStatic)

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
			// GET
			r.Get("/getConfig", handleAPIGetConfig)
			r.Get("/getCurrentDataPath", handleAPIGetCurrentDataPath)
			r.Get("/getLanguages", handleAPIGetLanguages)
			r.Get("/getRepositoryURL", handleAPIGetGitRepositoryURL)
			r.Get("/getAvailableFileViews", handleAPIGetAvailableFileViews)
			r.Get("/getCustomCSS", handleAPIGetCustomCSS)
			r.Get("/getDarkMode", handleAPIGetDarkMode)
			r.Get("/getColorSchemes", handleAPIGetColorSchemes)
			r.Get("/getDarkModeStatus", handleAPIGetDarkModeStatus)

			// POST
			r.Post("/setLanguage", handleAPISetLanguage)
			r.Post("/setRepositoryURL", handleAPISetGitRepositoryURL)
			r.Post("/setFileView", handleAPISetFileView)
			r.Post("/customCSS", handleCustomCSS)
			r.Post("/setDataPath", handleAPISetDataPath)
			r.Post("/setDarkMode", handleAPISetDarkMode)
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
	var basePath, filePath, fullPath string

	if strings.HasPrefix(r.URL.Path, "/static/") {
		basePath = "static"
		filePath = strings.TrimPrefix(r.URL.Path, "/static/")
		fullPath = filepath.ToSlash(filepath.Join(basePath, filePath))
	} else if strings.HasPrefix(r.URL.Path, "/themes/") {
		basePath = "themes"
		filePath = strings.TrimPrefix(r.URL.Path, "/themes/")
		fullPath = filepath.Join(basePath, filePath)
	} else {
		http.NotFound(w, r)
		return
	}

	fmt.Printf("handleStatic called for: %s (base: %s)\n", filePath, basePath)

	if basePath == "static" && strings.HasPrefix(filePath, "css/") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		cssFile := strings.TrimPrefix(filePath, "css/")

		if cssFile == "custom.css" {
			customCSS := configmanager.GetUserSettings().CustomCSS
			w.Write([]byte(customCSS))
			return
		}
	}

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

	if basePath == "themes" {
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			logging.LogDebug("theme file not found: %s", fullPath)
			http.NotFound(w, r)
			return
		}
		logging.LogDebug("serving theme file: %s", fullPath)
		http.ServeFile(w, r, fullPath)
	} else {
		data, err := staticFiles.ReadFile(fullPath)
		if err != nil {
			fmt.Printf("failed to read embedded file %s: %v\n", fullPath, err)
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}
}

func getViewName(templateName string) string {
	if templateName == "fileview" {
		return configmanager.GetFileView()
	}
	// todo: get from configmanager - usersettings for other templates
	return ""
}

// ----------------------------------------------------------------------------------------
// ------------------------------------ default routes ------------------------------------
// ----------------------------------------------------------------------------------------

func handleHome(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName("home")
	data := thememanager.NewBaseTemplateData("home")

	err := tm.Render(w, "home", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName("settings")
	data := thememanager.NewSettingsTemplateData()

	err := tm.Render(w, "settings", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName("admin")
	data := thememanager.NewBaseTemplateData("Admin")

	err := tm.Render(w, "admin", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handlePlayground(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName("playground")
	data := thememanager.NewBaseTemplateData("playground")

	err := tm.Render(w, "playground", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleLatestChanges(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName("latestchanges")
	data := thememanager.NewBaseTemplateData("latestchanges")

	err := tm.Render(w, "latestchanges", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName("history")
	data := thememanager.NewBaseTemplateData("history")

	err := tm.Render(w, "history", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleOverview(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName("overview")
	data := thememanager.NewBaseTemplateData("overview")

	err := tm.Render(w, "overview", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleSearchPage(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName("search")
	data := thememanager.NewBaseTemplateData("search")

	err := tm.Render(w, "search", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
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

	// query := fmt.Sprintf("%s:%s", metadataType, value)

	tm := thememanager.GetThemeManager()
	viewName := getViewName("browsefiles")
	data := thememanager.NewBaseTemplateData("browsefiles")

	err := tm.Render(w, "browsefiles", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleDashboardNew(w http.ResponseWriter, r *http.Request) {
	tm := thememanager.GetThemeManager()
	viewName := getViewName("dashboardnew")
	data := thememanager.NewBaseTemplateData("dashboardnew")

	err := tm.Render(w, "dashboardnew", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleDashboardEdit(w http.ResponseWriter, r *http.Request) {
	// id := chi.URLParam(r, "id")
	// dashboard, err := dashboard.Get(id)
	// if err != nil {
	// 	http.Error(w, "dashboard not found", http.StatusNotFound)
	// 	return
	// }

	tm := thememanager.GetThemeManager()
	viewName := getViewName("dashboardedit")
	data := thememanager.NewBaseTemplateData("dashboardedit")

	err := tm.Render(w, "dashboardedit", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleDashboardView(w http.ResponseWriter, r *http.Request) {
	// id := chi.URLParam(r, "id")
	// if id == "" {
	// 	id = "home"
	// }

	// dash, err := dashboard.Get(id)
	// if err != nil {
	// 	http.Error(w, "dashboard not found", http.StatusNotFound)
	// 	return
	// }

	tm := thememanager.GetThemeManager()
	viewName := getViewName("dashboardview")
	data := thememanager.NewBaseTemplateData("dashboardview")

	err := tm.Render(w, "dashboardview", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
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

	// For full page requests, render through template system
	tm := thememanager.GetThemeManager()
	viewName := getViewName("fileview")
	data := thememanager.NewFileViewTemplateData(filepath.Base(filePath), filePath, fileContent, viewName)

	// Always render through base template, not individual views
	err = tm.Render(w, "fileview", "", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}
}

func handleFileEdit(w http.ResponseWriter, r *http.Request) {
	// filePath := strings.TrimPrefix(r.URL.Path, "/files/edit/")
	// fullPath := utils.ToFullPath(filePath)
	//
	// content, err := files.GetRawContent(fullPath)
	// if err != nil {
	// 	content = ""
	// }

	tm := thememanager.GetThemeManager()
	viewName := getViewName("fileedit")
	data := thememanager.NewBaseTemplateData("fileedit")

	err := tm.Render(w, "fileedit", viewName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("error rendering template: %v", err), http.StatusInternalServerError)
		return
	}

}
